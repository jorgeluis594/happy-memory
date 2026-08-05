# Tarea 07: Diagnosticar y operar de forma confiable

Estado: planificada

Dependencias: tareas 01 a 06

## Contexto

La base global concentra los proyectos y memorias del usuario. El producto debe identificar problemas de acceso, compatibilidad o indexación antes de que causen resultados silenciosamente incorrectos, y debe responder de forma predecible cuando dos procesos compiten por escribir.

Este incremento completa las garantías operativas del POC y proporciona un diagnóstico no destructivo del almacenamiento.

## Resultado de producto

El usuario puede verificar la salud del almacenamiento y recibe errores JSON estables ante incompatibilidad, fallos o contención, sin estados parciales ni reparaciones inesperadas.

## Alcance

### Incluye

- `happy-memory doctor`.
- Comprobación de acceso al archivo de base de datos.
- Comprobación de compatibilidad de versión de esquema.
- Verificación básica de integridad SQLite.
- Verificación de claves foráneas y disponibilidad de FTS5.
- Comparación entre memorias activas y el índice de texto completo.
- Informe JSON estructurado de cada comprobación.
- Validación de compatibilidad de esquema antes de todos los comandos.
- Transacciones completas para todas las mutaciones.
- WAL, tiempo de espera y reintentos cortos y acotados ante contención de escritura.
- Contratos uniformes para errores estables del POC.
- Fechas UTC RFC 3339 y separación consistente entre `stdout` y `stderr`.

### Fuera de alcance

- Reparaciones automáticas o destructivas desde `doctor`.
- Purga, compactación o borrado físico.
- Recuperación de una base corrupta sin intervención del usuario.
- Replicación, alta disponibilidad o múltiples escritores simultáneos de SQLite.
- Stack traces en la salida normal.

## Criterios de aceptación de producto

1. En una base saludable, `doctor` devuelve `ok: true` y reporta como satisfactorios acceso, esquema, integridad, claves foráneas, FTS5 y consistencia del índice.
2. Si una comprobación falla, `doctor` identifica el problema y el estado no saludable en JSON, y no modifica la base.
3. Una versión de esquema incompatible impide que cualquier comando opere y produce un error estable sin intentar una migración destructiva.
4. Si falta o sobra una entrada FTS respecto de las memorias activas, `doctor` detecta la inconsistencia.
5. Una violación de claves foráneas o un fallo de integridad básico se reporta explícitamente.
6. Lectores pueden completar consultas durante una escritura corta en modo WAL.
7. Ante contención temporal, una escritura realiza únicamente reintentos cortos y acotados; si la contención persiste, devuelve `STORE_BUSY`.
8. Un `STORE_BUSY` no crea versiones, revisiones, asociaciones ni entradas FTS parciales.
9. Los errores funcionales usan cuando corresponde `PROJECT_NOT_INITIALIZED`, `VALIDATION_ERROR`, `MEMORY_NOT_FOUND`, `DUPLICATE_MEMORY` y `VERSION_CONFLICT`.
10. Los fallos operativos usan `STORE_BUSY` o `STORE_ERROR` sin exponer stack traces por defecto.
11. Todo éxito se escribe en `stdout` con `ok: true` y código de salida cero; todo error se escribe en `stderr` con `ok: false` y código distinto de cero.
12. Las respuestas del producto no mezclan texto humano fuera del documento JSON.
13. Todas las fechas expuestas usan UTC y formato RFC 3339.
14. La falla de una parte de una mutación revierte el estado actual, la revisión, las etiquetas y el índice como una sola unidad.

## Escenario integral de validación

1. Ejecutar `doctor` sobre una base saludable y conservar el informe JSON.
2. Introducir en un entorno de prueba una inconsistencia controlada del índice y confirmar que `doctor` la detecta sin repararla.
3. Ejecutar una lectura mientras otra instancia mantiene una escritura corta y comprobar que la lectura completa.
4. Mantener una contención superior al límite y confirmar `STORE_BUSY` sin cambios parciales.
5. Provocar un fallo entre dos efectos de una mutación y comprobar que ninguno queda persistido.
6. Recorrer al menos un ejemplo de cada código estable y validar canal de salida, código de proceso y forma JSON.

El escenario se aprueba cuando los fallos son visibles, deterministas y no comprometen la integridad del estado.

## Trazabilidad

- Especificación de producto: secciones 8.3, 8.4, 9, 10, 11 y criterios de aceptación de concurrencia e integridad.
- Arquitectura: la coordinación de transacciones y tecnología permanece en adaptadores y `internal/app`; las reglas de negocio no se desplazan fuera de sus módulos.
