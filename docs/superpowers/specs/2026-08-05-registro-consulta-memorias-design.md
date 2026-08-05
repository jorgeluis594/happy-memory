# Diseño: registrar y consultar memorias

## Objetivo y alcance

La Tarea 03 incorpora la creación, obtención y listado de memorias activas del proyecto Git actual. Cada memoria queda aislada por proyecto, auditada desde su creación, etiquetada e indexada en FTS5. Actualización, borrado, restauración, historial público, búsqueda textual y catálogo de etiquetas quedan fuera de alcance.

## Dominio y contratos

`internal/memory` contiene la entidad, los filtros, la normalización, el hash y los casos de uso. Los tipos válidos son `fact`, `decision`, `constraint`, `preference`, `procedure` y `lesson`; importancia y confianza están entre 1 y 5. Título y contenido normalizan CRLF/CR a LF y recortan espacios exteriores. El hash es SHA-256 de la serialización JSON determinista de `[type,title,content]`.

`happy-memory create --input -` consume exactamente un objeto JSON, rechaza campos desconocidos y contenido adicional. `attributes` puede omitirse, ser `null` o un objeto. Las etiquetas se recortan y normalizan en minúsculas; dos variantes equivalentes en el mismo payload son inválidas. La primera variante persistida se conserva como nombre canónico. `agent.name` y `agent.role` son textos libres, recortados y no vacíos cuando aparecen; el rol omitido se almacena como `unknown`.

`get` y `list` sólo ven filas activas del proyecto actual. `list` admite tipo, etiquetas repetibles con semántica AND y mínimos de importancia y confianza, y ordena por `updated_at DESC, id ASC`. Las respuestas públicas usan `version`, fechas UTC RFC 3339 con segundos y no exponen `project_id` ni procedencia.

## Persistencia

La migración Goose `00002` crea `memories`, `memory_revisions`, `tags`, `memory_tags` y la tabla virtual `memory_fts`. Las FKs compuestas incluyen `project_id` para impedir asociaciones entre proyectos. Un índice único parcial sobre `(project_id, content_hash)` evita duplicados activos.

Los repositorios reciben `*gorm.DB`; `database/sql` se conserva sólo para la conexión compartida y Goose. La creación se ejecuta con una transacción GORM: inserta memoria, crea o reutiliza etiquetas, crea asociaciones, guarda el snapshot completo de la revisión `create` e inserta FTS5. Cualquier error revierte todo. FTS5 usa `Exec` a través de la misma transacción GORM porque es una tabla virtual específica de SQLite.

## Proyecto y procedencia

El adaptador Git resuelve tanto el directorio común como la raíz absoluta del worktree actual. Un adaptador de composición combina el proyecto catalogado con esa raíz para el servicio de memorias. La revisión inicial conserva atributos, etiquetas canónicas, hash, nombre y rol libres del agente y ruta del worktree.

## Errores y pruebas

Entradas inválidas producen `VALIDATION_ERROR`; IDs ausentes o de otro proyecto, `MEMORY_NOT_FOUND`; duplicados, `DUPLICATE_MEMORY` con `details.memory_id`. Los fallos internos se presentan como `STORE_ERROR`. Los éxitos se escriben sólo en stdout y los errores sólo en stderr, siempre como JSON sin trazas ni logs GORM.

Las pruebas cubren normalización, validación, hash, procedencia, esquema y FTS5, atomicidad, duplicados por proyecto, reutilización canónica de etiquetas, aislamiento, filtros AND, orden estable, stdin estricto y JSON exacto. La aceptación final ejecuta `go test -race -cover ./...` y `make check`.
