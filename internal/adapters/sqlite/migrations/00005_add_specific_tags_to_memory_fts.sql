-- +goose Up
DROP TABLE memory_fts;
CREATE VIRTUAL TABLE memory_fts USING fts5(
    memory_id UNINDEXED,
    project_id UNINDEXED,
    title,
    content,
    specific_tags
);
INSERT INTO memory_fts (memory_id, project_id, title, content, specific_tags)
SELECT m.id, m.project_id, m.title, m.content,
       COALESCE((
           SELECT group_concat(normalized_name, ' ')
           FROM (
               SELECT t.normalized_name
               FROM memory_tags AS mt
               JOIN tags AS t
                 ON t.project_id = mt.project_id AND t.id = mt.tag_id
               WHERE mt.project_id = m.project_id AND mt.memory_id = m.id
               ORDER BY t.normalized_name
           )
       ), '')
FROM memories AS m
WHERE m.deleted_at IS NULL;

-- +goose Down
DROP TABLE memory_fts;
CREATE VIRTUAL TABLE memory_fts USING fts5(
    memory_id UNINDEXED,
    project_id UNINDEXED,
    title,
    content
);
INSERT INTO memory_fts (memory_id, project_id, title, content)
SELECT id, project_id, title, content
FROM memories
WHERE deleted_at IS NULL;
