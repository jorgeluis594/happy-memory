# Tarea 03: Registrar y consultar memorias

Estado: planificada

Dependencias: tareas 01 y 02

## Contexto

El primer valor funcional del producto aparece cuando un agente puede persistir una idea atómica y recuperarla más tarde dentro del mismo proyecto. Desde el inicio deben respetarse los tipos, las puntuaciones obligatorias, el aislamiento, la prevención de duplicados y la trazabilidad de la creación.

Este incremento entrega el flujo básico completo de captura y consulta. Las etiquetas incluidas en una memoria se conservan desde esta tarea, aunque los comandos para descubrir el vocabulario se entregan en la Tarea 05.

## Resultado de producto

El usuario puede crear una memoria válida, obtenerla por ID y explorar las memorias actuales de su proyecto sin ver datos de otros proyectos.

## Alcance

### Incluye

- Las migraciones del esquema requerido para memorias, revisiones, etiquetas, asociaciones y FTS5.
- `happy-memory create --input -` con JSON por `stdin`.
- Tipos `fact`, `decision`, `constraint`, `preference`, `procedure` y `lesson`.
- Validación de título, contenido, importancia, confianza, atributos, etiquetas y procedencia del agente.
- Cálculo interno y determinista de `content_hash`.
- Prevención de duplicados exactos activos dentro del mismo proyecto.
- Creación de la memoria en versión `1` y de su revisión `create` completa e inmutable.
- Asociación o creación de las etiquetas incluidas en la entrada.
- Indexación del estado activo en FTS5 dentro de la misma transacción.
- `happy-memory get <memory-id>`.
- `happy-memory list` con filtros por tipo, etiqueta, importancia y confianza.
- Orden estable del listado por actualización descendente e ID ascendente.
- Contratos JSON de éxito y error para estos comandos.

### Fuera de alcance

- Actualizar, eliminar o restaurar memorias.
- Consultar revisiones mediante `history`.
- Buscar texto o calcular ranking.
- Consultar o buscar el catálogo de etiquetas.
- Comparar equivalencia semántica entre memorias.

## Criterios de aceptación de producto

1. Una entrada válida crea una memoria con UUID público, versión `1`, fechas UTC RFC 3339 y todos los campos proporcionados.
2. La creación registra exactamente una revisión `create` cuyo snapshot representa por completo el estado resultante, incluidas etiquetas, atributos, procedencia y ruta del worktree.
3. `get` devuelve el estado actual completo de una memoria del proyecto activo.
4. `list` devuelve solamente memorias del proyecto activo y las ordena por `updated_at DESC` y luego por `memory_id ASC`.
5. Los filtros de listado por tipo, etiqueta, importancia y confianza excluyen los registros que no cumplen las condiciones.
6. Título o contenido vacíos después de recortar espacios producen `VALIDATION_ERROR` sin crear memoria ni revisión.
7. Importancia o confianza fuera de `1` a `5`, un tipo desconocido o atributos que no sean un objeto JSON válido producen `VALIDATION_ERROR`.
8. El fingerprint usa tipo, título y contenido normalizados según la especificación; etiquetas y atributos no cambian el resultado.
9. Intentar crear un duplicado exacto activo en el mismo proyecto devuelve `DUPLICATE_MEMORY` e identifica la memoria existente cuando está disponible.
10. La misma memoria puede crearse en otro proyecto sin ser tratada como duplicado.
11. Un ID inexistente o perteneciente a otro proyecto devuelve `MEMORY_NOT_FOUND` sin revelar datos del otro proyecto.
12. Si falla la memoria, su revisión, una etiqueta o el índice FTS, la creación completa se revierte.
13. El CLI deriva `content_hash`, ID, versión, fechas y ruta del worktree; una entrada no puede imponer esos valores.
14. Las respuestas exitosas se escriben como JSON en `stdout`; las fallidas, como JSON en `stderr`, sin stack trace por defecto.

## Escenario integral de validación

1. Inicializar dos proyectos.
2. Crear en el primero una memoria `decision` con atributos, dos etiquetas y procedencia de agente.
3. Recuperarla por ID y comprobar que coincide con la entrada y tiene versión `1`.
4. Listarla con filtros que coinciden y confirmar que desaparece con un filtro incompatible.
5. Intentar crear el mismo tipo, título y contenido con etiquetas distintas en el primer proyecto y comprobar `DUPLICATE_MEMORY`.
6. Crear esa misma memoria en el segundo proyecto y comprobar que se acepta.
7. Intentar consultar desde el segundo proyecto el ID del primero y comprobar `MEMORY_NOT_FOUND`.

El escenario se aprueba cuando la memoria es persistente, auditable y está aislada por proyecto desde su creación.

## Trazabilidad

- Especificación de producto: secciones 4, 5.3 a 5.7, 6.1, 6.5, 8, 9, 11 y criterios de aceptación de memorias.
- Arquitectura: módulo de memorias, puertos de persistencia y adaptadores de CLI y SQLite.
