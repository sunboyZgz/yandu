package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"yandu/internal/model"
	"yandu/internal/pathpolicy"
)

var Presets = []string{"thumbnail-256-v1", "webp-1280-v1", "video-cover-v1", "mp4-720-v1", "hls-720-v1"}

func validatePresets(presets []string) error {
	if len(presets) > 5 {
		return fail("INVALID_PRESET", "at most five presets", 400)
	}
	seen := map[string]bool{}
	for _, p := range presets {
		found := false
		for _, valid := range Presets {
			if valid == p {
				found = true
			}
		}
		if !found || seen[p] {
			return fail("INVALID_PRESET", p, 400)
		}
		seen[p] = true
	}
	return nil
}
func (s *Service) runJob() {
	if len(s.MediaCommand) == 0 {
		return
	}
	s.DB.Exec("UPDATE jobs SET state='queued' WHERE state='running' AND lease<?", time.Now().Unix())
	cancelError, _ := json.Marshal(&apiError{Code: "VERSION_CHANGED", Message: "source deleted or replaced"})
	s.DB.Exec("UPDATE jobs SET state='cancelled',error='VERSION_CHANGED',error_json=? WHERE state='queued' AND object IN (SELECT e.id FROM entries e WHERE e.deleted!=0 OR e.current_version!=jobs.version)", string(cancelError))
	s.DB.Exec("UPDATE jobs SET state='failed',error='attempt limit reached',error_json=? WHERE state='queued' AND attempts>=3", `{"code":"PROCESSING_INTERRUPTED","message":"retry limit reached"}`)
	var id, project, object, version, credential, preset string
	e := s.DB.QueryRow("SELECT j.id,j.project,j.object,j.version,j.credential,j.preset FROM jobs j JOIN entries e ON e.id=j.object WHERE j.state='queued' AND j.attempts<3 AND e.deleted=0 AND e.current_version=j.version AND NOT EXISTS(SELECT 1 FROM media_candidates c WHERE c.job=j.id) ORDER BY j.rowid LIMIT 1").Scan(&id, &project, &object, &version, &credential, &preset)
	if e != nil {
		return
	}
	res, e := s.DB.Exec("UPDATE jobs SET state='running',attempts=attempts+1,lease=? WHERE id=? AND state='queued'", time.Now().Add(30*time.Second).Unix(), id)
	if e != nil {
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return
	}
	e = s.executeJob(id, project, object, version, credential, preset)
	if e != nil {
		s.recordJobError(id, e)
	}

}
func (s *Service) executeJob(id, project, object, version, credential, preset string) (jobErr error) {
	s.mediaMu.Lock()
	s.mediaActive[id] = true
	s.mediaMu.Unlock()
	defer func() { s.mediaMu.Lock(); delete(s.mediaActive, id); s.mediaMu.Unlock() }()
	pr, e := s.project(project)
	if e != nil {
		return e
	}
	if _, e = s.checkVolume(pr); e != nil {
		return e
	}
	p, e := s.credentialPrincipal(project, credential)
	if e != nil {
		return fail("CREDENTIAL_REVOKED", "initiating credential no longer valid", 401)
	}
	if e = s.allowed(s.ctx, p, "process"); e != nil {
		return e
	}
	var source, mime, sourceSHA string
	var size, stamp int64
	e = s.DB.QueryRow("SELECT path,mime,size,stamp,sha256 FROM versions WHERE project=? AND id=? AND object=?", project, version, object).Scan(&source, &mime, &size, &stamp, &sourceSHA)
	if e != nil {
		return e
	}
	if e = s.checkFormal(pr, source, size, stamp); e != nil {
		return e
	}
	var constraintJSON, processingVersion string
	if e = s.DB.QueryRow("SELECT constraints_json,processing_version FROM jobs WHERE id=?", id).Scan(&constraintJSON, &processingVersion); e != nil {
		return e
	}
	var constraint OutputConstraint
	if e = json.Unmarshal([]byte(constraintJSON), &constraint); e != nil {
		return e
	}
	if processingVersion == "legacy-v0.2" {
		processingVersion = s.ProcessingVersion
		s.DB.Exec("UPDATE jobs SET processing_version=? WHERE id=?", processingVersion, id)
	}
	if processingVersion != s.ProcessingVersion || s.mediaFingerprint() != processingVersion {
		return fail("PROCESSING_VERSION_CHANGED", "worker pipeline changed; submit a new logical task", 409)
	}
	budget := size*4 + (512 << 20)
	s.mu.Lock()
	tx, e := s.DB.Begin()
	if e != nil {
		s.mu.Unlock()
		return e
	}
	var used, reserved int64
	tx.QueryRow("SELECT COALESCE((SELECT SUM(size) FROM versions WHERE project=?),0)+COALESCE((SELECT SUM(size) FROM derivatives WHERE project=?),0)", project, project).Scan(&used)
	tx.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM reservations WHERE project=?", project).Scan(&reserved)
	volume, e := s.checkVolume(pr)
	var pooled int64
	tx.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM reservations WHERE pool=?", pr.Pool).Scan(&pooled)
	if e != nil || used+reserved+budget > pr.Quota || pooled+budget+(64<<20) > volume.Free {
		tx.Rollback()
		s.mu.Unlock()
		return fail("INSUFFICIENT_STORAGE", "media workspace reservation failed", 507)
	}
	_, e = tx.Exec("INSERT OR REPLACE INTO reservations VALUES(?,?,?,?)", id, project, pr.Pool, budget)
	if e == nil {
		e = tx.Commit()
	} else {
		tx.Rollback()
	}
	s.mu.Unlock()
	if e != nil {
		return e
	}
	defer func() {
		s.DB.Exec("DELETE FROM reservations WHERE id=? AND NOT EXISTS(SELECT 1 FROM media_candidates WHERE job=?)", id, id)
	}()
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Hour)
	defer cancel()
	defer func() {
		if jobErr != nil && ctx.Err() != nil && s.ctx.Err() == nil {
			var state string
			s.DB.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&state)
			if state == "cancelled" {
				jobErr = fail("JOB_CANCELLED", "job cancelled", 409)
				return
			}
			v, err := s.entry(project, object, false)
			if err != nil || v.Version != version {
				jobErr = fail("VERSION_CHANGED", "source deleted or replaced", 409)
				return
			}
			if err = s.allowed(s.ctx, p, "process"); err != nil {
				jobErr = err
				return
			}
			if _, err = s.checkVolume(pr); err != nil {
				jobErr = err
				return
			}
			if ctx.Err() == context.DeadlineExceeded {
				jobErr = fail("PROCESSING_TIMEOUT", "media processing time budget exhausted", 503)
			}
		}
	}()
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				var jobState string
				if s.DB.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&jobState) != nil || jobState != "running" {
					cancel()
					return
				}
				v, err := s.entry(project, object, false)
				if err != nil || v.Version != version || s.allowed(ctx, p, "process") != nil {
					cancel()
					return
				}
				if _, err = s.checkVolume(pr); err != nil {
					cancel()
					return
				}
				s.DB.Exec("UPDATE jobs SET lease=? WHERE id=? AND state='running'", time.Now().Add(30*time.Second).Unix(), id)
			}
		}
	}()
	dir := filepath.Join(pr.Root, "temporary", "jobs", id)
	if e = pathpolicy.NoLinks(filepath.Dir(dir)); e != nil {
		return e
	}
	os.RemoveAll(dir)
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	response, e := s.Backend.Download(ctx, pr.Key, source, "")
	if e != nil {
		return e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("original download HTTP %d", response.StatusCode)
	}
	f, e := os.OpenFile(filepath.Join(dir, "input"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	sourceHash := sha256.New()
	n, e := io.CopyBuffer(io.MultiWriter(f, sourceHash), io.LimitReader(response.Body, size+1), make([]byte, 64<<10))
	f.Close()
	if e != nil || n != size {
		return fmt.Errorf("original length mismatch: %v", e)
	}
	if hex.EncodeToString(sourceHash.Sum(nil)) != sourceSHA {
		return fail("SOURCE_HASH_MISMATCH", "source bytes do not match registered version", 409)
	}
	payload, _ := json.Marshal(map[string]any{"directory": dir, "preset": preset, "mime": mime, "maxOutputBytes": budget - size, "outputConstraint": constraint, "processingVersion": processingVersion})
	cmd := exec.CommandContext(ctx, s.MediaCommand[0], s.MediaCommand[1:]...)
	if len(s.MediaEnvironment) > 0 {
		cmd.Env = []string{}
		for _, v := range os.Environ() {
			key, _, _ := strings.Cut(v, "=")
			if _, override := s.MediaEnvironment[key]; !override {
				cmd.Env = append(cmd.Env, v)
			}
		}
		for key, value := range s.MediaEnvironment {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	cmd.Stdin = bytes.NewReader(payload)
	var out, stderr limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	cmd.WaitDelay = 2 * time.Second
	configureCommand(cmd)
	var manifest struct {
		Files []struct {
			Name string `json:"name"`
			Mime string `json:"mime"`
		} `json:"files"`
		Result *MediaResult `json:"result,omitempty"`
		Error  *apiError    `json:"error,omitempty"`
	}
	runError := cmd.Run()
	parseError := json.Unmarshal(out.Bytes(), &manifest)
	if runError != nil {
		if pathpolicy.NoLinks(s.Dir) == nil {
			logFile := filepath.Join(s.Dir, "media-worker-errors.log")
			if pathpolicy.NoLinks(logFile) == nil {
				if log, e := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); e == nil {
					fmt.Fprintf(log, "job %s: %s\n", id, stderr.String())
					log.Close()
				}
			}
		}
		if manifest.Error != nil && parseError == nil {
			return manifest.Error
		}
		return fmt.Errorf("media worker: %v: %s", runError, stderr.String())
	}
	if parseError != nil {
		return fmt.Errorf("invalid worker manifest: %w", parseError)
	}
	if preset == "mp4-720-v1" {
		if manifest.Result == nil {
			return fail("INVALID_MEDIA_RESULT", "MP4 worker omitted measured result", 503)
		}
		r := manifest.Result
		if r.FullCompressedBytes <= 0 || r.OutputBytes <= 0 || r.OriginalDurationSeconds <= 0 || r.OutputDurationSeconds <= 0 || r.OutputDurationSeconds > r.OriginalDurationSeconds+0.25 || r.Mime != "video/mp4" || r.RetainedRange.StartSeconds != 0 || r.RetainedRange.EndSeconds != r.OutputDurationSeconds {
			return fail("INVALID_MEDIA_RESULT", "invalid MP4 result measurements", 503)
		}
		if constraint.MaxFileBytes > 0 && r.OutputBytes > constraint.MaxFileBytes {
			return &apiError{Code: "OUTPUT_LIMIT_EXCEEDED", Message: "final MP4 exceeds project file limit", Status: 422, Details: map[string]any{"maxFileBytes": constraint.MaxFileBytes, "outputBytes": r.OutputBytes}}
		}
		if r.Trimmed && (constraint.MaxFileBytes == 0 || constraint.OnOverflow != "keep-start") {
			return fail("INVALID_MEDIA_RESULT", "worker trimmed without explicit authorization", 503)
		}
		if constraint.MaxFileBytes > 0 && ((r.FullCompressedBytes <= constraint.MaxFileBytes && r.Trimmed) || (r.FullCompressedBytes > constraint.MaxFileBytes && !r.Trimmed)) {
			return fail("INVALID_MEDIA_RESULT", "trim decision does not match full output measurement", 503)
		}
	}
	if len(manifest.Files) == 0 || len(manifest.Files) > 10000 {
		return fmt.Errorf("invalid media output count")
	}
	type output struct {
		id, path, mime, sha string
		size                int64
		stamp               int64
	}
	outputs := []output{}
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, v := range outputs {
			var registered int
			s.DB.QueryRow("SELECT COUNT(*) FROM derivatives WHERE project=? AND path=?", project, v.path).Scan(&registered)
			if registered == 0 {
				if s.Backend.Remove(s.ctx, pr.Key, v.path) == nil {
					s.DB.Exec("DELETE FROM media_candidates WHERE job=? AND path=?", id, v.path)
				}
			}
		}
	}()
	var total int64
	for _, item := range manifest.Files {
		if item.Name == "input" || strings.ContainsAny(item.Name, "\\\x00") || path.Clean(item.Name) != item.Name || strings.HasPrefix(item.Name, "/") || strings.Contains(item.Name, "..") {
			return fmt.Errorf("invalid worker output path")
		}
		local := filepath.Join(dir, filepath.FromSlash(item.Name))
		if e = pathpolicy.NoLinks(local); e != nil {
			return e
		}
		f, e = os.Open(local)
		if e != nil {
			return e
		}
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() {
			f.Close()
			return fmt.Errorf("nonregular output")
		}
		total += info.Size()
		if total*2+size > budget {
			f.Close()
			return fail("WORKSPACE_LIMIT", "media output exceeds reservation", 507)
		}
		h := sha256.New()
		if _, e = io.CopyBuffer(h, f, make([]byte, 64<<10)); e != nil {
			f.Close()
			return e
		}
		sha := hex.EncodeToString(h.Sum(nil))
		if _, e = f.Seek(0, 0); e != nil {
			f.Close()
			return e
		}
		if preset == "mp4-720-v1" && item.Name == "video.mp4" {
			if item.Mime != "video/mp4" || manifest.Result.OutputBytes != info.Size() || manifest.Result.OutputSHA256 != sha {
				f.Close()
				return fail("INVALID_MEDIA_RESULT", "MP4 measurements do not match output bytes", 503)
			}
		}
		target := "derived/" + object + "/" + version + "/" + preset + "/" + id + "/" + item.Name
		if e = s.mkdirs(ctx, pr.Key, path.Dir(target)); e != nil {
			f.Close()
			return e
		}
		outputs = append(outputs, output{model.ID(), target, item.Mime, sha, info.Size(), 0})
		if _, e = s.DB.Exec("INSERT OR IGNORE INTO media_candidates VALUES(?,?,?)", id, project, target); e != nil {
			f.Close()
			return e
		}
		e = s.Backend.Upload(ctx, pr.Key, target, f, info.Size())
		f.Close()
		if e != nil {
			return e
		}
		if e = s.verify(ctx, pr, target, sha, info.Size()); e != nil {
			return e
		}
		stamp, e := s.formalStamp(pr, target, info.Size())
		if e != nil {
			return e
		}
		outputs[len(outputs)-1].stamp = stamp
	}
	if preset == "mp4-720-v1" {
		for i, item := range manifest.Files {
			if item.Name == "video.mp4" {
				manifest.Result.DerivativeID = outputs[i].id
			}
		}
		if manifest.Result.DerivativeID == "" {
			return fail("INVALID_MEDIA_RESULT", "MP4 output missing", 503)
		}
	}
	if _, e = s.checkVolume(pr); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e = s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.validCommit(tx, p, "process"); e != nil {
		return e
	}
	var current string
	var deleted int
	tx.QueryRow("SELECT current_version,deleted FROM entries WHERE id=? AND project=?", object, project).Scan(&current, &deleted)
	if current != version || deleted != 0 {
		return fail("VERSION_CHANGED", "job output cannot restore a deleted or replaced original", 409)
	}
	var jobState string
	if e = tx.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&jobState); e != nil {
		return e
	}
	if jobState != "running" {
		return fail("JOB_CANCELLED", "job no longer running", 409)
	}
	var otherReserved int64
	if e = tx.QueryRow("SELECT COALESCE((SELECT SUM(size) FROM versions WHERE project=?),0)+COALESCE((SELECT SUM(size) FROM derivatives WHERE project=?),0)", project, project).Scan(&used); e != nil {
		return e
	}
	if e = tx.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM reservations WHERE project=? AND id!=?", project, id).Scan(&otherReserved); e != nil {
		return e
	}
	var quota int64
	if e = tx.QueryRow("SELECT quota FROM projects WHERE id=?", project).Scan(&quota); e != nil {
		return e
	}
	if used+otherReserved+total > quota {
		return fail("INSUFFICIENT_STORAGE", "quota changed before media commit", 507)
	}
	for _, v := range outputs {
		if _, e = tx.Exec("INSERT OR IGNORE INTO derivatives(id,project,object,version,preset,path,size,mime,sha256,stamp,job) VALUES(?,?,?,?,?,?,?,?,?,?,?)", v.id, project, object, version, preset, v.path, v.size, v.mime, v.sha, v.stamp, id); e != nil {
			return e
		}
	}
	if _, e = tx.Exec("DELETE FROM media_candidates WHERE job=?", id); e != nil {
		return e
	}
	resultJSON := ""
	if manifest.Result != nil {
		raw, _ := json.Marshal(manifest.Result)
		resultJSON = string(raw)
	}
	if _, e = tx.Exec("UPDATE jobs SET state='succeeded',error='',error_json='',result_json=?,source_sha256=?,lease=0 WHERE id=? AND state='running'", resultJSON, sourceSHA, id); e != nil {
		return e
	}
	if _, e = tx.Exec("DELETE FROM reservations WHERE id=?", id); e != nil {
		return e
	}
	e = tx.Commit()
	committed = e == nil
	return e
}

// Interrupted or failed workers leave a durable cleanup record and keep their reservation.
// Cleanup is internal and remains possible after the initiating credential was revoked.
func (s *Service) recoverMediaCleanup() {
	rows, e := s.DB.Query("SELECT c.job,c.project,c.path FROM media_candidates c JOIN jobs j ON j.id=c.job WHERE j.state!='running' OR j.lease<?", time.Now().Unix())
	if e != nil {
		return
	}
	type item struct{ job, project, path string }
	items := []item{}
	for rows.Next() {
		var v item
		if rows.Scan(&v.job, &v.project, &v.path) == nil {
			items = append(items, v)
		}
	}
	rows.Close()
	for _, v := range items {
		s.mediaMu.Lock()
		active := s.mediaActive[v.job]
		s.mediaMu.Unlock()
		if active {
			continue
		}
		pr, e := s.project(v.project)
		if e != nil {
			continue
		}
		if _, e = s.checkVolume(pr); e != nil {
			continue
		}
		var registered int
		s.DB.QueryRow("SELECT COUNT(*) FROM derivatives WHERE project=? AND path=?", v.project, v.path).Scan(&registered)
		ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
		if registered > 0 || s.Backend.Remove(ctx, pr.Key, v.path) == nil {
			s.DB.Exec("DELETE FROM media_candidates WHERE job=? AND path=?", v.job, v.path)
			s.DB.Exec("DELETE FROM reservations WHERE id=? AND NOT EXISTS(SELECT 1 FROM media_candidates WHERE job=?)", v.job, v.job)
		}
		cancel()
	}
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 65536 {
		keep := 65536 - b.Len()
		if keep > len(p) {
			keep = len(p)
		}
		b.Buffer.Write(p[:keep])
	}
	return n, nil
}
