-- +goose Up
CREATE TABLE projects (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    last_known_git_common_dir TEXT NOT NULL CHECK (length(trim(last_known_git_common_dir)) > 0),
    created_at TEXT NOT NULL CHECK (length(trim(created_at)) > 0),
    updated_at TEXT NOT NULL CHECK (length(trim(updated_at)) > 0)
);

CREATE INDEX projects_name_id_idx ON projects (name, id);

-- +goose Down
DROP TABLE projects;
