-- 0001_create_schema.up.sql
CREATE TYPE pr_status AS ENUM ('OPEN','MERGED');

CREATE TABLE IF NOT EXISTS teams (
    team_name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
    user_id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS pull_requests (
    pull_request_id TEXT PRIMARY KEY,
    pull_request_name TEXT NOT NULL,
    author_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    status pr_status NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    merged_at TIMESTAMP WITH TIME ZONE NULL
);

CREATE TABLE IF NOT EXISTS pr_reviewers (
    pull_request_id TEXT REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
    user_id TEXT REFERENCES users(user_id) ON DELETE RESTRICT,
    PRIMARY KEY (pull_request_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_pr_reviewers_user ON pr_reviewers(user_id);
CREATE INDEX IF NOT EXISTS idx_users_team_active ON users(team_name, is_active);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

