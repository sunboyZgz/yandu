-- +goose Up
CREATE TABLE media_candidates (job TEXT NOT NULL REFERENCES jobs(id), project TEXT NOT NULL REFERENCES projects(id), path TEXT NOT NULL, PRIMARY KEY(job,path));
-- +goose Down
DROP TABLE media_candidates;
