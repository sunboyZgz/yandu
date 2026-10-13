-- +goose Up
ALTER TABLE projects ADD COLUMN mp4_max_file_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN policy_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE uploads ADD COLUMN processing_snapshot TEXT NOT NULL DEFAULT '{}';
ALTER TABLE derivatives ADD COLUMN job TEXT NOT NULL DEFAULT '';
CREATE TABLE jobs_v2 (
 id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), object TEXT NOT NULL,
 version TEXT NOT NULL, credential TEXT NOT NULL, preset TEXT NOT NULL, state TEXT NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0, lease INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '',
 constraints_json TEXT NOT NULL DEFAULT '{}', policy_revision INTEGER NOT NULL DEFAULT 0,
 processing_version TEXT NOT NULL DEFAULT 'legacy-v0.2', source_sha256 TEXT NOT NULL DEFAULT '',
 result_json TEXT NOT NULL DEFAULT '', error_json TEXT NOT NULL DEFAULT '', upload_id TEXT NOT NULL DEFAULT '',
 spec_hash TEXT NOT NULL, UNIQUE(project,credential,spec_hash)
);
INSERT INTO jobs_v2(id,project,object,version,credential,preset,state,attempts,lease,error,spec_hash,source_sha256)
 SELECT j.id,j.project,j.object,j.version,j.credential,j.preset,j.state,j.attempts,j.lease,j.error,'legacy:'||j.id,COALESCE(v.sha256,'') FROM jobs j LEFT JOIN versions v ON v.id=j.version AND v.project=j.project;
CREATE TABLE media_candidates_saved AS SELECT * FROM media_candidates;
DROP TABLE media_candidates;
DROP TABLE jobs;
ALTER TABLE jobs_v2 RENAME TO jobs;
CREATE TABLE media_candidates (job TEXT NOT NULL REFERENCES jobs(id), project TEXT NOT NULL REFERENCES projects(id), path TEXT NOT NULL, PRIMARY KEY(job,path));
INSERT INTO media_candidates SELECT * FROM media_candidates_saved;
DROP TABLE media_candidates_saved;
CREATE TABLE media_submissions (project TEXT NOT NULL REFERENCES projects(id), credential TEXT NOT NULL, idempotency TEXT NOT NULL, request_hash TEXT NOT NULL, jobs_json TEXT NOT NULL, PRIMARY KEY(project,credential,idempotency));
CREATE INDEX jobs_upload ON jobs(project,upload_id);
CREATE INDEX derivatives_job ON derivatives(project,job);
-- +goose Down
-- This migration preserves several outputs for one version/preset. A downgrade cannot
-- safely restore the old unique constraint; restore a consistent pre-upgrade backup.
SELECT yandu_downgrade_requires_pre_upgrade_backup();
