# Tarea 04: Gestionar el ciclo de vida y el historial

Estado: planificada

Dependencias: tareas 01 a 03

## Contexto

Las memorias cambian a medida que evoluciona el proyecto, pero un agente no debe sobrescribir silenciosamente una edición ajena ni perder el estado anterior. El borrado debe ser reversible y cada mutación debe quedar representada por un snapshot completo.

Este incremento completa el ciclo de vida de una memoria y convierte las versiones en un contrato observable de concurrencia e historial.

## Resultado de producto

El usuario puede modificar, eliminar y restaurar memorias con control optimista, inspeccionar toda su historia y recuperar estados anteriores sin rebobinar el contador de versiones.

## Alcance

### Incluye

- `happy-memory update <memory-id> --expected-version <n> --input -`.
- Semántica de patch para campos ausentes, `attributes: null` y reemplazo completo de etiquetas.
- Detección de actualizaciones sin cambios.
- Control optimista mediante `expected-version`.
- Revisión completa e inmutable para cada actualización efectiva.
- `happy-memory delete <memory-id> --expected-version <n>` como borrado lógico.
- Exclusión de memorias eliminadas de consultas normales e índice FTS.
- `--include-deleted` para `get` y `list`.
- `happy-memory restore <memory-id> --version <n> --expected-version <n>`.
- Restauración del snapshot elegido como una versión nueva.
- `happy-memory history <memory-id>` ordenado por versión.
- Actualización atómica de estado, revisión, etiquetas e índice FTS.

### Fuera de alcance

- Borrado físico o purga automática de memorias y revisiones.
- Editar o eliminar revisiones históricas.
- Estados `archived` o `superseded`.
- Resolver automáticamente dos cambios en conflicto.
- Restaurar una versión de un proyecto distinto.

## Criterios de aceptación de producto

1. Un patch con la versión esperada correcta modifica únicamente los campos presentes, incrementa la versión una vez y crea una revisión `update` completa.
2. Un campo ausente conserva su valor; `attributes: null` elimina los atributos; `tags: []` elimina todas las etiquetas; un campo `tags` presente reemplaza el conjunto completo.
3. Un patch cuyo estado resultante es idéntico no cambia `updated_at`, no incrementa la versión y no crea una revisión.
4. Dos actualizaciones basadas en la misma versión no pueden sobrescribirse: la primera puede completar y la segunda devuelve `VERSION_CONFLICT` con la versión actual.
5. Una actualización que produciría un duplicado activo devuelve `DUPLICATE_MEMORY` y no modifica ningún dato.
6. `delete` con la versión correcta asigna `deleted_at`, incrementa la versión, crea una revisión `delete` y retira la memoria de listados normales y FTS.
7. Una memoria eliminada no puede actualizarse ni eliminarse de nuevo; debe restaurarse primero.
8. `get` y `list` solo devuelven una memoria eliminada cuando reciben `--include-deleted`.
9. `restore` copia el snapshot histórico seleccionado, restaura sus etiquetas y atributos, incrementa el contador desde la versión actual y crea una revisión `restore`.
10. Restaurar no elimina ni modifica las revisiones anteriores y no reutiliza el número de la versión restaurada.
11. Una restauración que produciría un duplicado activo falla sin alterar la memoria eliminada.
12. `history` devuelve todas las revisiones de la memoria del proyecto activo, ordenadas por versión y con snapshots completos e inmutables.
13. Un `expected-version` incorrecto devuelve `VERSION_CONFLICT` e incluye la versión actual.
14. Un fallo al persistir cualquier parte de una mutación revierte también el estado, la versión, la revisión, las etiquetas y FTS.
15. Las fechas de las revisiones usan UTC RFC 3339 y la procedencia disponible queda preservada en cada snapshot.

## Escenario integral de validación

1. Crear una memoria con dos etiquetas y versión `1`.
2. Actualizar únicamente el contenido con `expected-version 1` y confirmar la versión `2`.
3. Repetir el mismo patch y confirmar que continúa en versión `2`.
4. Intentar otro cambio con `expected-version 1` y comprobar `VERSION_CONFLICT`.
5. Eliminarla con versión `2` y confirmar que deja de aparecer en consultas normales.
6. Consultarla con `--include-deleted` y revisar las tres revisiones existentes.
7. Restaurar el snapshot de versión `1`; confirmar una nueva versión `4`, el contenido y etiquetas originales, y cuatro revisiones preservadas.

El escenario se aprueba cuando ninguna mutación pierde historia, mezcla proyectos o deja un estado parcial.

## Trazabilidad

- Especificación de producto: secciones 5.5, 6.2 a 6.5, 8, 9, 11 y criterios de aceptación de memorias y concurrencia.
- Arquitectura: los casos de uso de mutación permanecen en el dominio; SQLite implementa la transacción requerida por sus puertos.
