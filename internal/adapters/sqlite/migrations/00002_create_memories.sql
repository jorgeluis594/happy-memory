-- +goose Up
CREATE TABLE memories (
    id TEXT NOT NULL CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 1 CHECK (current_version >= 1),
    type TEXT NOT NULL CHECK (type IN ('fact', 'decision', 'constraint', 'preference', 'procedure', 'lesson')),
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    content TEXT NOT NULL CHECK (length(trim(content)) > 0),
    importance INTEGER NOT NULL CHECK (importance BETWEEN 1 AND 5),
    confidence INTEGER NOT NULL CHECK (confidence BETWEEN 1 AND 5),
    attributes_json TEXT,
    content_hash TEXT NOT NULL CHECK (length(content_hash) = 64),
    created_at TEXT NOT NULL CHECK (length(trim(created_at)) > 0),
    updated_at TEXT NOT NULL CHECK (length(trim(updated_at)) > 0),
    deleted_at TEXT,
    PRIMARY KEY (id),
    UNIQUE (project_id, id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CHECK (attributes_json IS NULL OR json_valid(attributes_json))
);

CREATE UNIQUE INDEX memories_active_content_hash_idx
    ON memories (project_id, content_hash) WHERE deleted_at IS NULL;
CREATE INDEX memories_project_updated_idx
    ON memories (project_id, updated_at DESC, id ASC) WHERE deleted_at IS NULL;

CREATE TABLE memory_revisions (
    memory_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version >= 1),
    operation TEXT NOT NULL CHECK (length(trim(operation)) > 0),
    snapshot_json TEXT NOT NULL CHECK (json_valid(snapshot_json)),
    agent_name TEXT,
    agent_role TEXT NOT NULL CHECK (length(trim(agent_role)) > 0),
    worktree_root TEXT NOT NULL CHECK (length(trim(worktree_root)) > 0),
    created_at TEXT NOT NULL CHECK (length(trim(created_at)) > 0),
    PRIMARY KEY (memory_id, version),
    FOREIGN KEY (project_id, memory_id) REFERENCES memories(project_id, id) ON DELETE CASCADE
);

CREATE TABLE tags (
    id TEXT NOT NULL CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    normalized_name TEXT NOT NULL CHECK (length(trim(normalized_name)) > 0),
    created_at TEXT NOT NULL CHECK (length(trim(created_at)) > 0),
    PRIMARY KEY (id),
    UNIQUE (project_id, id),
    UNIQUE (project_id, normalized_name),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE memory_tags (
    project_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    PRIMARY KEY (memory_id, tag_id),
    FOREIGN KEY (project_id, memory_id) REFERENCES memories(project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, tag_id) REFERENCES tags(project_id, id) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE memory_fts USING fts5(
    memory_id UNINDEXED,
    project_id UNINDEXED,
    title,
    content,
    tags
);

-- +goose Down
DROP TABLE memory_fts;
DROP TABLE memory_tags;
DROP TABLE tags;
DROP TABLE memory_revisions;
DROP TABLE memories;
