# Plan secuencial del POC de happy-memory

Fecha: 2026-08-05

Estado: secuencia aprobada

## Objetivo

Este directorio convierte la especificación del POC en incrementos verticales de producto. Cada tarea entrega un comportamiento observable y aprobable mediante la CLI; no representa solamente una capa técnica ni un conjunto de funciones internas.

El plan cubre el POC completo descrito en:

- `docs/memory-cli-product-design.md`, que define el comportamiento y los contratos del producto.
- `docs/architecture.md`, que define los límites y las reglas de implementación del repositorio.

## Cómo usar este plan

Las tareas se ejecutan en orden. Una tarea puede asumir que los criterios de aceptación de las tareas anteriores ya se cumplen y debe preservar esos comportamientos como regresión.

Cada tarea contiene:

- El contexto y el resultado de producto esperado.
- Sus dependencias.
- El alcance incluido y excluido.
- Criterios de aceptación observables a nivel de producto.
- Un escenario integral de validación.
- Trazabilidad hacia la especificación.

Una tarea se considera terminada cuando todos sus criterios y su escenario integral pasan, los comportamientos entregados anteriormente continúan funcionando y no quedan decisiones funcionales abiertas dentro de su alcance.

## Distinción entre las dos inicializaciones

El plan separa explícitamente dos responsabilidades:

1. **Inicialización del almacenamiento:** un `init` válido crea y configura de manera segura la base SQLite local del repositorio, compartida por sus worktrees.
2. **Inicialización de un proyecto:** el mismo comando vincula el repositorio Git actual con un `project_id` y lo registra en esa base.

Los demás comandos solo abren una base existente y nunca crean almacenamiento implícitamente. No se agrega un comando público distinto para el bootstrap.

## Secuencia

| Orden | Tarea | Resultado aprobable |
| --- | --- | --- |
| 1 | [Inicializar el CLI y su almacenamiento](01-inicializacion-del-cli-y-almacenamiento.md) | Un `init` válido prepara o actualiza la base local del repositorio sin pérdida de datos. |
| 2 | [Inicializar e identificar proyectos](02-inicializacion-e-identidad-del-proyecto.md) | Un repositorio Git obtiene una identidad estable, compartida por sus worktrees y aislada de otros clones. |
| 3 | [Registrar y consultar memorias](03-registro-y-consulta-de-memorias.md) | El usuario puede crear, obtener y listar memorias válidas dentro del proyecto activo. |
| 4 | [Gestionar el ciclo de vida y el historial](04-ciclo-de-vida-e-historial.md) | Las memorias pueden evolucionar, eliminarse y restaurarse sin perder historia ni sobrescribir cambios concurrentes. |
| 5 | [Gestionar el vocabulario de etiquetas](05-vocabulario-de-etiquetas.md) | El usuario puede descubrir y reutilizar etiquetas normalizadas dentro de cada proyecto. |
| 6 | [Buscar con ranking explicable](06-busqueda-y-ranking-explicable.md) | La búsqueda devuelve resultados aislados, filtrados, ordenados de forma determinista y con puntuación explicada. |
| 7 | [Diagnosticar y operar de forma confiable](07-diagnostico-y-confiabilidad-operativa.md) | El producto detecta problemas de almacenamiento y responde de manera estable ante fallos y contención. |
| 8 | [Validar integralmente el POC](08-validacion-integral-del-poc.md) | El flujo completo satisface la especificación en repositorios, worktrees y clones antes de aceptar el POC. |

## Cobertura de comandos

| Comando | Tarea principal |
| --- | --- |
| `happy-memory init [--name <name>]` | 02 |
| `happy-memory project show` | 02 |
| `happy-memory projects list` | 02 |
| `happy-memory create --input -` | 03 |
| `happy-memory get <memory-id> [--include-deleted]` | 03 y 04 |
| `happy-memory list [filters] [--include-deleted]` | 03 y 04 |
| `happy-memory update <memory-id> --expected-version <n> --input -` | 04 |
| `happy-memory delete <memory-id> --expected-version <n>` | 04 |
| `happy-memory restore <memory-id> --version <n> --expected-version <n>` | 04 |
| `happy-memory history <memory-id>` | 04 |
| `happy-memory tags list` | 05 |
| `happy-memory tags search <query> [--limit <n>]` | 05 |
| `happy-memory search <query> [filters]` | 06 |
| `happy-memory doctor` | 07 |

## Reglas transversales

- Todos los comandos producen exclusivamente el contrato JSON definido por el producto.
- Toda operación normal resuelve el proyecto desde el repositorio Git actual; no acepta selección manual de otro proyecto.
- El aislamiento por `project_id` se mantiene desde la primera tarea que manipula datos de proyecto.
- Las mutaciones preservan de forma atómica el estado actual, las revisiones, las etiquetas y el índice de búsqueda que correspondan.
- Los criterios describen resultados de producto. La implementación debe respetar adicionalmente las dependencias y límites de `docs/architecture.md`.
- Las capacidades excluidas por la especificación —embeddings, sincronización remota, cifrado, purga automática, operaciones batch e interpretación semántica— permanecen fuera de todas las tareas.
