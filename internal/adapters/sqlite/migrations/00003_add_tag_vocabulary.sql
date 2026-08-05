-- +goose Up
ALTER TABLE tags ADD COLUMN description TEXT;
ALTER TABLE tags ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
UPDATE tags SET updated_at = created_at;

ALTER TABLE memory_tags ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
UPDATE memory_tags
SET created_at = (
    SELECT tags.created_at
    FROM tags
    WHERE tags.project_id = memory_tags.project_id
      AND tags.id = memory_tags.tag_id
);

-- +goose Down
ALTER TABLE memory_tags DROP COLUMN created_at;
ALTER TABLE tags DROP COLUMN updated_at;
ALTER TABLE tags DROP COLUMN description;
