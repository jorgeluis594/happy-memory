# Diseño: vocabulario de etiquetas

Fecha: 2026-08-05

Estado: aprobado, pendiente de revisión escrita

## Objetivo

Completar el vocabulario reutilizable de etiquetas por proyecto. El CLI debe aceptar etiquetas en el formato histórico de string y en el formato enriquecido con descripción, reutilizar variantes normalizables, preservar la primera descripción y ofrecer consultas con conteos de memorias activas.

## Alcance

El incremento incluye:

- Normalización determinista y validación de nombres de etiquetas.
- Entrada compatible con strings y objetos `{name, description}` en `create` y `update`.
- Reutilización de etiquetas por `(project_id, normalized_name)`.
- Conservación de la descripción de una etiqueta existente.
- Comandos `tags list` y `tags search <query>`.
- Conteos de memorias activas y búsqueda case-insensitive por nombre o descripción.
- Aislamiento por proyecto en asociaciones y consultas.
- Semántica AND para filtros `--tag` repetidos.

No incluye renombrado, fusión, eliminación, jerarquías, sinónimos ni búsqueda semántica de etiquetas.

## Contrato de entrada y salida

`create` y el campo presente `tags` de `update` aceptan ambos formatos dentro del mismo arreglo:

```json
{
  "tags": [
    "git",
    {"name": "Git Operations", "description": "Operaciones e identidad Git"}
  ]
}
```

Un string equivale a un objeto con `name` y sin descripción. La salida siempre usa objetos canónicos y expone `id`, `name`, `normalized_name`, `description`, `created_at` y `updated_at`. Las respuestas de vocabulario añaden `active_memory_count`.

Los objetos de entrada rechazan campos desconocidos. `description` puede omitirse o ser `null`; cuando está presente como string se recortan sus espacios externos y un resultado vacío se trata como ausencia de descripción.

Dos elementos de una misma petición cuyo nombre normaliza al mismo valor producen `VALIDATION_ERROR`, incluso si usan formatos distintos. Un nombre que normaliza a vacío también produce `VALIDATION_ERROR`.

## Reglas de dominio

La normalización:

1. Recorta espacios externos.
2. Convierte a minúsculas.
3. Reemplaza cada secuencia de espacios internos por un guion.
4. Colapsa guiones repetidos.

El nombre canónico almacenado será el nombre de la primera creación, recortado externamente. Al reutilizar una etiqueta, ni el nombre canónico ni la descripción cambian. Una descripción solo se aplica al insertar una etiqueta nueva.

`tags list` devuelve todas las etiquetas del proyecto, incluidas las que tengan conteo activo cero, ordenadas por `normalized_name` ascendente. `tags search` busca el query como texto literal, sin distinguir mayúsculas, en el nombre canónico y la descripción; ordena por `active_memory_count DESC, normalized_name ASC`. Un query vacío después de recortar produce `VALIDATION_ERROR`.

Los conteos consideran solo asociaciones con memorias cuyo `deleted_at` es nulo. Delete y restore no reescriben asociaciones: el conteo cambia por el estado de la memoria.

Los filtros de memoria normalizan cada `--tag`, rechazan filtros vacíos o equivalentes repetidos y exigen que una memoria posea todas las etiquetas solicitadas.

## Arquitectura

El módulo `internal/memory` seguirá siendo dueño de las entidades y reglas de etiquetas porque las etiquetas ya participan en creación, actualización, restauración, snapshots y filtros de memorias. Se ampliará su puerto de repositorio con consultas de vocabulario y su servicio con casos de uso `TagsList` y `TagsSearch`.

El adaptador SQLite resolverá o creará etiquetas dentro de la misma transacción que cada mutación de memoria. Usará la unicidad `(project_id, normalized_name)` para reutilización segura y las claves foráneas compuestas existentes para impedir asociaciones entre proyectos. Las consultas de vocabulario siempre incluirán `project_id` y calcularán conteos activos mediante `LEFT JOIN` para conservar etiquetas sin uso activo.

El adaptador Cobra decodificará el formato dual, expondrá el grupo `tags` y presentará respuestas JSON estables. `internal/app` no requiere una dependencia nueva porque el servicio de memoria ya está conectado.

## Persistencia y migración

Una migración Goose posterior a `00002` agregará:

- `tags.description`, anulable.
- `tags.updated_at`, no anulable y poblado inicialmente desde `created_at`.
- `memory_tags.created_at`, no anulable y poblado con un timestamp determinista compatible con los datos existentes.

La migración conservará IDs, etiquetas y asociaciones existentes. Su `Down` reconstruirá las tablas cuando SQLite no pueda retirar las columnas conservando las restricciones; no eliminará otras tablas del módulo.

## Flujo transaccional

Para cada create, update o restore que establezca etiquetas:

1. El dominio normaliza y valida toda la colección.
2. SQLite abre la transacción de mutación existente.
3. Por cada etiqueta busca `(project_id, normalized_name)`.
4. Si existe, reutiliza sus datos persistidos e ignora nombre y descripción nuevos.
5. Si no existe, la crea con ID, nombre canónico, descripción y timestamps suministrados por dominio.
6. Reemplaza las asociaciones de la memoria cuando corresponde.
7. Persiste memoria, revisión y FTS como una única unidad atómica.

Cualquier violación de proyecto o fallo intermedio revierte la transacción completa.

## Errores

Errores de nombre, descripción, query, forma JSON o duplicados equivalentes en una petición se presentan como `VALIDATION_ERROR`. Fallos de integridad o almacenamiento no atribuibles a entrada pública se presentan como `STORE_ERROR`. Las consultas nunca aceptan un `project_id` del usuario; resuelven el proyecto activo.

## Pruebas

Las pruebas de dominio cubrirán normalización, formato dual, descripciones opcionales, duplicados equivalentes y validación del query. Las pruebas SQLite cubrirán reutilización, primera descripción, aislamiento, restricciones cruzadas, conteos tras delete/restore, búsqueda por descripción, orden y etiquetas con conteo cero. Las pruebas Cobra cubrirán los comandos, JSON, ambos formatos de entrada y errores estables. Las pruebas existentes de filtros verificarán explícitamente AND con nombres normalizados.

El escenario integral de `docs/poc/05-vocabulario-de-etiquetas.md` se ejecutará como prueba de aceptación del adaptador SQLite/CLI.
