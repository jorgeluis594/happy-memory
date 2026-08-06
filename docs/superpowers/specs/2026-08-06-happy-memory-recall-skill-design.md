# Diseño del skill `happy-memory-recall`

## Objetivo

Crear un skill local que recupere memorias relevantes antes de realizar trabajo sustantivo en el repositorio o cuando un usuario o agente solicite una consulta de memoria. El skill traducirá la intención de la tarea a consultas y filtros soportados por `happy-memory`, refinará la recuperación de forma acotada y entregará las memorias sin resumirlas ni decidir cómo aplicarlas.

El skill vivirá en `.agents/skills/happy-memory-recall/` y evolucionará junto con el contrato del CLI.

## Activación y límites

El skill se activará automáticamente antes de diseñar, implementar, modificar, diagnosticar o tomar decisiones sobre el repositorio. También se activará cuando el usuario o el agente solicite explícitamente recordar, recuperar contexto, consultar decisiones previas o buscar memoria.

No se activará automáticamente para operaciones mecánicas cuyo resultado no dependa del contexto acumulado, como mostrar estado, formatear archivos o ejecutar una prueba ya indicada. Una solicitud explícita siempre podrá activarlo.

Su única responsabilidad será la recuperación. No creará, actualizará, eliminará, restaurará ni resumirá memorias. Tampoco decidirá cómo aplicar el contexto recuperado a la tarea posterior, inicializará proyectos ni reparará el almacenamiento.

## Estructura

```text
.agents/skills/happy-memory-recall/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    └── search-cli.md
```

`SKILL.md` contendrá la lógica de recuperación: activación operativa, construcción de consultas, selección progresiva de filtros, refinamiento, deduplicación, criterios de parada, entrega y guardrails.

`references/search-cli.md` será la fuente técnica del contrato de lectura. Documentará la sintaxis de `search`, `tags search` y `tags list`; tipos y filtros; normalización de etiquetas; semántica textual; límites; ranking; JSON público; restricciones; errores y ejemplos. `SKILL.md` ordenará leer esta referencia antes de construir la primera consulta de cada recuperación. La lógica no se duplicará en la referencia.

`agents/openai.yaml` ofrecerá metadatos coherentes con el nombre, descripción y prompt predeterminado del skill. No se crearán scripts, assets ni documentación auxiliar.

## Flujo de recuperación

1. Leer `references/search-cli.md` y ejecutar desde el repositorio al que pertenece la tarea.
2. Extraer de la tarea uno o dos términos literales de alta señal que probablemente aparezcan en el título o contenido de una memoria. No enviar la solicitud completa, porque los términos de una consulta se combinan con AND.
3. Ejecutar una primera búsqueda amplia con `--limit 10`. Omitir tipo, etiquetas y mínimos salvo que la intención de la solicitud los justifique inequívocamente.
4. Evaluar la utilidad de los primeros resultados:
   - Detenerse cuando cubran directamente el tema y otra consulta no prometa mejorar materialmente el contexto.
   - Ante cero resultados, simplificar la consulta o probar un sinónimo mediante otra ejecución.
   - Ante demasiado ruido, descubrir vocabulario de etiquetas y añadir filtros progresivamente.
   - Ante conceptos alternativos, ejecutar consultas separadas para simular OR.
5. Ejecutar como máximo tres comandos `search` por recuperación. Las consultas auxiliares de etiquetas no consumirán ese presupuesto.
6. Deduplicar memorias por ID. Cuando una memoria aparezca en más de una consulta, conservar una copia y registrar todas las coincidencias.
7. No comparar ni agregar scores provenientes de consultas distintas. Cada score y posición conservarán el contexto de su intento.
8. Terminar con uno de los estados `found`, `empty` o `failed`.

La estrategia prioriza recall en el primer intento y precisión durante el refinamiento. La iteración queda acotada para evitar que la recuperación domine la tarea principal.

## Política de filtros

### Tipo

Usar `--type` solo cuando la tarea busque inequívocamente una clase de memoria:

| Tipo | Intención |
| --- | --- |
| `fact` | Estado o comportamiento vigente |
| `decision` | Elección previa y sus motivos |
| `constraint` | Regla o límite obligatorio |
| `preference` | Convención deseada |
| `procedure` | Secuencia reutilizable |
| `lesson` | Problema conocido, hallazgo o aprendizaje |

Cuando varias clases puedan ser relevantes, omitir el tipo en la primera consulta o ejecutar consultas separadas en vez de forzar un tipo incorrecto.

### Etiquetas

Aplicar `--tag` solamente cuando el usuario proporcione la etiqueta o esta haya sido confirmada mediante `tags search`. Preferir `tags search` para descubrir vocabulario y reservar `tags list` para solicitudes que requieran inspeccionar el catálogo completo. Recordar que varias etiquetas tienen semántica AND.

### Importancia y confianza

No aplicar mínimos por defecto. Reservar valores de 4 o 5 para solicitudes que pidan expresamente contexto crítico, de alto impacto, confirmado o altamente confiable. Importancia y confianza son dimensiones independientes.

### Límite

Comenzar con `--limit 10`. Aumentarlo, sin superar 100, solo cuando una consulta amplia ya produzca resultados relevantes pero insuficientes.

### Capacidades inexistentes

No intentar expresar mediante flags filtros por fecha, versión, ID, rama, worktree, atributos, autor, máximos, OR, NOT, exclusión de etiquetas, varios tipos ni memorias eliminadas. OR se aproximará mediante consultas separadas; las demás capacidades permanecerán fuera del skill.

## Entrega estructurada

El skill entregará al agente consumidor un paquete YAML con el siguiente contrato:

```yaml
status: found | empty | failed
attempts:
  - query: "sqlite retry"
    filters:
      type: lesson
      tags: []
      min_importance: null
      min_confidence: null
      limit: 10
    ranking_version: 1
    result_count: 2
memories:
  - id: "..."
    type: lesson
    title: "..."
    content: "..."
    tags: ["sqlite"]
    importance: 4
    confidence: 5
    matches:
      - attempt: 1
        rank: 1
        score:
          final: 0.92
          text: 1
          importance: 0.75
          confidence: 1
error: null
```

Título y contenido se preservarán sin resumen. Cada intento registrará la consulta, filtros, versión de ranking y número de resultados. Cada coincidencia registrará su intento, posición y componentes de score. Las memorias de consultas distintas conservarán el orden de descubrimiento y no se reordenarán comparando scores.

`found` exige al menos una memoria recuperada. `empty` representa ejecuciones exitosas sin coincidencias después del refinamiento. `failed` incluye un objeto `error` con `code`, `message` y `details` cuando estén disponibles. En los otros estados, `error` será nulo.

## Errores

- Si el ejecutable `happy-memory` no está disponible, terminar con `failed` y explicar la dependencia ausente.
- Ante `GIT_REPOSITORY_NOT_FOUND`, `PROJECT_NOT_INITIALIZED`, `STORE_BUSY` o `STORE_ERROR`, terminar con `failed`. No inicializar, mutar, reparar ni presentar el fallo como ausencia de memorias.
- Ante `VALIDATION_ERROR` causado por un comando construido por el skill, corregir la invocación una sola vez. La ejecución corregida cuenta dentro del máximo de tres comandos `search`. Si el error persiste, terminar con `failed`.
- Una ejecución exitosa con `results: []` es la única base para considerar vacío un intento.

## Validación

La implementación se validará con el validador estructural de `skill-creator` y con pruebas de uso realistas que cubran:

1. Activación automática antes de una tarea sustantiva.
2. Activación por una solicitud explícita de memoria.
3. Búsqueda inicial amplia con resultados suficientes.
4. Simplificación o sinónimos después de un resultado vacío.
5. Descubrimiento y aplicación de etiquetas ante resultados ruidosos.
6. Aplicación justificada de tipo, importancia o confianza.
7. Simulación de OR, deduplicación y preservación del ranking por consulta.
8. Distinción entre `empty` y errores operativos.
9. Respeto del máximo de tres búsquedas y de los límites de alcance.

Las pruebas comprobarán la transferencia del comportamiento a solicitudes nuevas sin proporcionar al evaluador las decisiones esperadas. No ejecutarán mutaciones ni operarán sobre sistemas de producción.

## Criterios de aceptación

1. La metadata activa el skill para trabajo sustantivo y solicitudes explícitas, sin reclamar tareas mecánicas por defecto.
2. `SKILL.md` contiene la lógica de recuperación y remite al contrato técnico sin duplicarlo.
3. La referencia coincide con los comandos, filtros, semántica, salida y errores implementados por el CLI.
4. La primera consulta prioriza recall y los filtros se añaden solo con una justificación observable.
5. Ninguna recuperación supera tres ejecuciones de `search`.
6. Las consultas alternativas no mezclan scores y las memorias repetidas se deduplican por ID.
7. La entrega conserva el contenido original y toda la información necesaria para rastrear cómo se obtuvo cada memoria.
8. Un resultado vacío nunca oculta un fallo del CLI.
9. El skill no ejecuta comandos de mutación ni amplía su responsabilidad a interpretar o aplicar las memorias.
10. La carpeta pasa la validación oficial de `skill-creator` y las pruebas realistas satisfacen el flujo aprobado.
