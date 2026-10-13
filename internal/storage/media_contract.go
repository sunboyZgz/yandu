package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"yandu/internal/model"
)

const MediaPipelineVersion = "mp4-720-constraints-v1"

type OverflowRequest struct {
	OnOverflow string `json:"onOverflow"`
}
type ProcessingInput struct {
	Presets           []string                   `json:"presets"`
	OutputConstraints map[string]OverflowRequest `json:"outputConstraints,omitempty"`
}
type OutputConstraint struct {
	MaxFileBytes int64  `json:"maxFileBytes,omitempty"`
	OnOverflow   string `json:"onOverflow,omitempty"`
}
type ProcessingSnapshot struct {
	Constraints       map[string]OutputConstraint `json:"constraints"`
	PolicyRevision    int64                       `json:"policyRevision"`
	ProcessingVersion string                      `json:"processingVersion"`
}
type ProcessRequest struct {
	Object            string                     `json:"objectId"`
	Expected          string                     `json:"expectedVersionId,omitempty"`
	Presets           []string                   `json:"presets"`
	OutputConstraints map[string]OverflowRequest `json:"outputConstraints,omitempty"`
}
type JobReference struct {
	ID              string `json:"id"`
	Preset          string `json:"preset"`
	SourceVersionID string `json:"sourceVersionId"`
}
type MediaResult struct {
	DerivativeID            string  `json:"derivativeId,omitempty"`
	OriginalDurationSeconds float64 `json:"originalDurationSeconds"`
	FullCompressedBytes     int64   `json:"fullCompressedBytes"`
	OutputDurationSeconds   float64 `json:"outputDurationSeconds"`
	OutputBytes             int64   `json:"outputBytes"`
	OutputSHA256            string  `json:"outputSha256"`
	Mime                    string  `json:"mime"`
	Trimmed                 bool    `json:"trimmed"`
	RetainedRange           struct {
		StartSeconds float64 `json:"startSeconds"`
		EndSeconds   float64 `json:"endSeconds"`
	} `json:"retainedRange"`
}
type JobDetail struct {
	ID                 string            `json:"id"`
	State              string            `json:"state"`
	ObjectID           string            `json:"objectId"`
	SourceVersionID    string            `json:"sourceVersionId"`
	SourceSHA256       string            `json:"sourceSha256,omitempty"`
	Preset             string            `json:"preset"`
	ProcessingVersion  string            `json:"processingVersion"`
	PolicyRevision     int64             `json:"policyRevision"`
	AppliedConstraints *OutputConstraint `json:"appliedConstraints,omitempty"`
	Attempts           int               `json:"attempts"`
	Result             *MediaResult      `json:"result,omitempty"`
	Error              *apiError         `json:"error,omitempty"`
}

func (s *Service) mediaFingerprint() string {
	if len(s.MediaCommand) == 0 {
		return MediaPipelineVersion + ":disabled"
	}
	h := sha256.New()
	io.WriteString(h, MediaPipelineVersion)
	for _, arg := range s.MediaCommand {
		io.WriteString(h, filepath.Base(arg))
	}
	paths := append([]string{}, s.MediaCommand...)
	for _, arg := range s.MediaCommand {
		if strings.HasSuffix(arg, "worker.mjs") {
			paths = append(paths, filepath.Join(filepath.Dir(arg), "mp4.mjs"), filepath.Join(filepath.Dir(arg), "package-lock.json"))
		}
	}
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		key := "YANDU_" + strings.ToUpper(name)
		tool := s.MediaEnvironment[key]
		if tool == "" {
			tool = os.Getenv(key)
		}
		if tool == "" {
			for _, arg := range s.MediaCommand {
				if strings.HasSuffix(arg, "worker.mjs") {
					suffix := ""
					if runtime.GOOS == "windows" {
						suffix = ".exe"
					}
					candidate := filepath.Join(filepath.Dir(arg), "..", "bin", name+suffix)
					if _, e := os.Stat(candidate); e == nil {
						tool = candidate
						break
					}
				}
			}
		}
		if tool == "" {
			tool, _ = exec.LookPath(name)
		}
		if tool != "" {
			paths = append(paths, tool)
		}
	}
	for _, p := range paths {
		if f, e := os.Open(p); e == nil {
			info, e := f.Stat()
			if e == nil && info.Mode().IsRegular() {
				io.Copy(h, f)
			}
			f.Close()
		}
	}
	return MediaPipelineVersion + ":" + hex.EncodeToString(h.Sum(nil))[:16]
}
func (s *Service) processingSnapshot(pr Project, v ProcessingInput) (ProcessingSnapshot, error) {
	snap := ProcessingSnapshot{Constraints: map[string]OutputConstraint{}, PolicyRevision: pr.PolicyRevision, ProcessingVersion: s.ProcessingVersion}
	if e := validatePresets(v.Presets); e != nil {
		return snap, e
	}
	selected := map[string]bool{}
	for _, p := range v.Presets {
		selected[p] = true
	}
	for preset, r := range v.OutputConstraints {
		if preset != "mp4-720-v1" || !selected[preset] {
			return snap, fail("UNSUPPORTED_OUTPUT_CONSTRAINT", "constraints only apply to a selected mp4-720-v1 preset", 422)
		}
		if pr.MP4MaxFileBytes == 0 {
			return snap, fail("PROJECT_MEDIA_POLICY_REQUIRED", "configure the project's MP4 file limit before requesting output constraints", 422)
		}
		if r.OnOverflow != "reject" && r.OnOverflow != "keep-start" {
			return snap, fail("INVALID_OVERFLOW_POLICY", "onOverflow must be reject or keep-start", 422)
		}
	}
	if selected["mp4-720-v1"] && pr.MP4MaxFileBytes > 0 {
		policy := "reject"
		if r, ok := v.OutputConstraints["mp4-720-v1"]; ok {
			policy = r.OnOverflow
		}
		snap.Constraints["mp4-720-v1"] = OutputConstraint{pr.MP4MaxFileBytes, policy}
	}
	return snap, nil
}
func canonicalProcess(v ProcessRequest) string {
	v.Presets = append([]string{}, v.Presets...)
	sort.Strings(v.Presets)
	data, _ := json.Marshal(v)
	return digest(string(data))
}
func (s *Service) insertJobs(tx *sql.Tx, p Principal, object, version, upload, logical string, presets []string, snap ProcessingSnapshot) ([]JobReference, error) {
	var sha string
	if e := tx.QueryRow("SELECT sha256 FROM versions WHERE project=? AND object=? AND id=?", p.Project, object, version).Scan(&sha); e != nil {
		return nil, e
	}
	refs := []JobReference{}
	sorted := append([]string{}, presets...)
	sort.Strings(sorted)
	for _, preset := range sorted {
		constraint, _ := json.Marshal(snap.Constraints[preset])
		spec, _ := json.Marshal([]any{p.Project, p.ID, logical, object, version, preset, snap.ProcessingVersion, snap.PolicyRevision, json.RawMessage(constraint)})
		hash := digest(string(spec))
		id := model.ID()
		_, e := tx.Exec("INSERT OR IGNORE INTO jobs(id,project,object,version,credential,preset,state,constraints_json,policy_revision,processing_version,source_sha256,upload_id,spec_hash) VALUES(?,?,?,?,?,?,'queued',?,?,?,?,?,?)", id, p.Project, object, version, p.ID, preset, string(constraint), snap.PolicyRevision, snap.ProcessingVersion, sha, upload, hash)
		if e != nil {
			return nil, e
		}
		if e = tx.QueryRow("SELECT id FROM jobs WHERE project=? AND credential=? AND spec_hash=?", p.Project, p.ID, hash).Scan(&id); e != nil {
			return nil, e
		}
		refs = append(refs, JobReference{id, preset, version})
	}
	return refs, nil
}
func (s *Service) submitMediaJobs(ctx context.Context, p Principal, v ProcessRequest, key string) ([]JobReference, error) {
	if len(s.MediaCommand) == 0 {
		return nil, fail("MEDIA_UNAVAILABLE", "media worker unavailable", 503)
	}
	if e := validatePresets(v.Presets); e != nil {
		return nil, e
	}
	if len(v.Presets) == 0 {
		return nil, fail("INVALID_PRESET", "at least one preset required", 400)
	}
	if len(v.OutputConstraints) > 0 && (v.Expected == "" || key == "") {
		return nil, fail("PROCESS_PRECONDITION_REQUIRED", "expectedVersionId and Idempotency-Key required for constraint requests", 400)
	}
	if len(key) > 128 {
		return nil, fail("INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must not exceed 128 characters", 400)
	}
	requestHash := canonicalProcess(v)
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if e = s.validCommit(tx, p, "process"); e != nil {
		return nil, e
	}
	if key != "" {
		var old, raw string
		e = tx.QueryRow("SELECT request_hash,jobs_json FROM media_submissions WHERE project=? AND credential=? AND idempotency=?", p.Project, p.ID, key).Scan(&old, &raw)
		if e == nil {
			if old != requestHash {
				return nil, fail("IDEMPOTENCY_CONFLICT", "key already used for another processing request", 409)
			}
			var refs []JobReference
			e = json.Unmarshal([]byte(raw), &refs)
			return refs, e
		}
		if e != sql.ErrNoRows {
			return nil, e
		}
	}
	var version string
	e = tx.QueryRow("SELECT current_version FROM entries WHERE id=? AND project=? AND kind='file' AND deleted=0", v.Object, p.Project).Scan(&version)
	if e != nil {
		return nil, fail("NOT_FOUND", "source object not found", 404)
	}
	if version == "" {
		return nil, fail("NOT_READY", "original not ready", 409)
	}
	if v.Expected != "" && v.Expected != version {
		return nil, fail("VERSION_CHANGED", "expected source version no longer current", 409)
	}
	var pr Project
	pr.ID = p.Project
	if e = tx.QueryRow("SELECT mp4_max_file_bytes,policy_revision FROM projects WHERE id=?", p.Project).Scan(&pr.MP4MaxFileBytes, &pr.PolicyRevision); e != nil {
		return nil, e
	}
	snap, e := s.processingSnapshot(pr, ProcessingInput{v.Presets, v.OutputConstraints})
	if e != nil {
		return nil, e
	}
	logical := key
	if logical == "" {
		logical = "legacy:" + requestHash
	}
	refs, e := s.insertJobs(tx, p, v.Object, version, "", logical, v.Presets, snap)
	if e != nil {
		return nil, e
	}
	if key != "" {
		raw, _ := json.Marshal(refs)
		if _, e = tx.Exec("INSERT INTO media_submissions VALUES(?,?,?,?,?)", p.Project, p.ID, key, requestHash, string(raw)); e != nil {
			return nil, e
		}
	}
	return refs, tx.Commit()
}
func (s *Service) uploadJobs(project, upload string) []JobReference {
	refs := []JobReference{}
	rows, e := s.DB.Query("SELECT id,preset,version FROM jobs WHERE project=? AND upload_id=? ORDER BY preset", project, upload)
	if e != nil {
		return refs
	}
	defer rows.Close()
	for rows.Next() {
		var r JobReference
		if rows.Scan(&r.ID, &r.Preset, &r.SourceVersionID) == nil {
			refs = append(refs, r)
		}
	}
	return refs
}
func (s *Service) jobDetail(project, id string) (JobDetail, error) {
	var j JobDetail
	var c, result, errJSON, legacyError string
	e := s.DB.QueryRow("SELECT id,state,object,version,source_sha256,preset,processing_version,policy_revision,attempts,constraints_json,result_json,error_json,error FROM jobs WHERE project=? AND id=?", project, id).Scan(&j.ID, &j.State, &j.ObjectID, &j.SourceVersionID, &j.SourceSHA256, &j.Preset, &j.ProcessingVersion, &j.PolicyRevision, &j.Attempts, &c, &result, &errJSON, &legacyError)
	if e != nil {
		return j, fail("NOT_FOUND", "job not found", 404)
	}
	var constraint OutputConstraint
	json.Unmarshal([]byte(c), &constraint)
	if constraint.MaxFileBytes > 0 {
		j.AppliedConstraints = &constraint
	}
	if result != "" {
		json.Unmarshal([]byte(result), &j.Result)
	}
	if errJSON != "" {
		json.Unmarshal([]byte(errJSON), &j.Error)
	} else if legacyError != "" {
		j.Error = &apiError{Code: "PROCESSING_FAILED", Message: "processing failed; inspect local operation logs"}
	}
	return j, nil
}
func (s *Service) cancelJob(ctx context.Context, p Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.validCommit(tx, p, "process"); e != nil {
		return e
	}
	var state string
	if e = tx.QueryRow("SELECT state FROM jobs WHERE project=? AND id=?", p.Project, id).Scan(&state); e != nil {
		return fail("NOT_FOUND", "job not found", 404)
	}
	if state == "queued" || state == "running" {
		raw, _ := json.Marshal(&apiError{Code: "JOB_CANCELLED", Message: "job cancelled"})
		_, e = tx.Exec("UPDATE jobs SET state='cancelled',error='JOB_CANCELLED',error_json=?,lease=0 WHERE project=? AND id=?", string(raw), p.Project, id)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
func jobError(e error) *apiError {
	var a *apiError
	if errors.As(e, &a) {
		return a
	}
	return &apiError{Code: "PROCESSING_FAILED", Message: "media processing failed; inspect protected local logs", Status: 503}
}
func terminalMediaError(e error) bool {
	var a *apiError
	if !errors.As(e, &a) {
		return false
	}
	switch a.Code {
	case "OUTPUT_LIMIT_EXCEEDED", "NO_VALID_TRIM", "UNSUPPORTED_MEDIA", "INVALID_MEDIA_RESULT", "SOURCE_HASH_MISMATCH", "VERSION_CHANGED", "OBJECT_DELETED", "CREDENTIAL_REVOKED", "PROJECT_DISABLED", "JOB_CANCELLED", "WORKSPACE_LIMIT", "PROCESSING_VERSION_CHANGED", "PROCESSING_TIMEOUT":
		return true
	}
	return false
}
func (s *Service) recordJobError(id string, e error) {
	a := jobError(e)
	raw, _ := json.Marshal(a)
	state := "queued"
	var attempts int
	s.DB.QueryRow("SELECT attempts FROM jobs WHERE id=?", id).Scan(&attempts)
	if terminalMediaError(e) || attempts >= 3 {
		state = "failed"
	}
	if a.Code == "JOB_CANCELLED" || a.Code == "CREDENTIAL_REVOKED" || a.Code == "PROJECT_DISABLED" || a.Code == "VERSION_CHANGED" || a.Code == "OBJECT_DELETED" {
		state = "cancelled"
	}
	if s.ctx.Err() != nil && !terminalMediaError(e) {
		state = "queued"
	}
	s.DB.Exec("UPDATE jobs SET state=?,error=?,error_json=?,lease=0 WHERE id=? AND state='running'", state, e.Error(), string(raw), id)
}
func (s *Service) mediaCapabilities(pr Project) map[string]any {
	presets := map[string]any{}
	for _, preset := range Presets {
		entry := map[string]any{"supportsMaxFileBytes": false, "overflowPolicies": []string{}}
		if preset == "mp4-720-v1" {
			var limit any
			if pr.MP4MaxFileBytes > 0 {
				limit = pr.MP4MaxFileBytes
			}
			entry = map[string]any{"supportsMaxFileBytes": len(s.MediaCommand) > 0, "constraintScope": "project", "configuredMaxFileBytes": limit, "maxAllowedFileBytes": min(pr.Quota, int64(9007199254740991)), "overflowPolicies": []string{"reject", "keep-start"}, "policyRevision": pr.PolicyRevision}
		}
		presets[preset] = entry
	}
	return map[string]any{"version": 1, "presets": presets}
}
