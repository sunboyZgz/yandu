-- +goose Up
CREATE TABLE projects (id TEXT PRIMARY KEY, root TEXT NOT NULL UNIQUE, quota INTEGER NOT NULL CHECK(quota>0), enabled INTEGER NOT NULL DEFAULT 0, backend_key TEXT NOT NULL, volume TEXT NOT NULL, pool TEXT NOT NULL, quarantined INTEGER NOT NULL DEFAULT 0);
CREATE TABLE credentials (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), digest TEXT NOT NULL UNIQUE, scopes TEXT NOT NULL, revoked INTEGER NOT NULL DEFAULT 0, created INTEGER NOT NULL);
CREATE TABLE entries (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), parent TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('folder','file')), current_version TEXT NOT NULL DEFAULT '', deleted INTEGER NOT NULL DEFAULT 0, generation INTEGER NOT NULL DEFAULT 1, created INTEGER NOT NULL);
CREATE UNIQUE INDEX sibling_names ON entries(project,parent,name) WHERE deleted=0;
CREATE INDEX listing ON entries(project,parent,deleted,id);
CREATE TABLE versions (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), object TEXT NOT NULL REFERENCES entries(id), size INTEGER NOT NULL, sha256 TEXT NOT NULL, mime TEXT NOT NULL, path TEXT NOT NULL, created INTEGER NOT NULL);
CREATE TABLE uploads (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), credential TEXT NOT NULL, parent TEXT NOT NULL, name TEXT NOT NULL, object TEXT NOT NULL, version TEXT NOT NULL, expected TEXT NOT NULL, length INTEGER NOT NULL CHECK(length>=0), state TEXT NOT NULL, presets TEXT NOT NULL, request_hash TEXT NOT NULL, idempotency TEXT NOT NULL, error TEXT NOT NULL DEFAULT '', created INTEGER NOT NULL, UNIQUE(project,credential,idempotency));
CREATE TABLE reservations (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), pool TEXT NOT NULL, bytes INTEGER NOT NULL CHECK(bytes>=0));
CREATE TABLE operations (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), object TEXT NOT NULL, kind TEXT NOT NULL, state TEXT NOT NULL, source TEXT NOT NULL, target TEXT NOT NULL, error TEXT NOT NULL DEFAULT '');
CREATE TABLE jobs (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), object TEXT NOT NULL, version TEXT NOT NULL, credential TEXT NOT NULL, preset TEXT NOT NULL, state TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, lease INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '', UNIQUE(version,preset));
CREATE TABLE derivatives (id TEXT PRIMARY KEY, project TEXT NOT NULL REFERENCES projects(id), object TEXT NOT NULL, version TEXT NOT NULL, preset TEXT NOT NULL, path TEXT NOT NULL, size INTEGER NOT NULL, mime TEXT NOT NULL, sha256 TEXT NOT NULL, UNIQUE(version,preset,path));
CREATE TABLE outbox (id TEXT PRIMARY KEY, project TEXT NOT NULL, event TEXT NOT NULL, payload TEXT NOT NULL, created INTEGER NOT NULL);
-- +goose Down
DROP TABLE outbox;
DROP TABLE derivatives;
DROP TABLE jobs;
DROP TABLE operations;
DROP TABLE reservations;
DROP TABLE uploads;
DROP TABLE versions;
DROP TABLE entries;
DROP TABLE credentials;
DROP TABLE projects;
