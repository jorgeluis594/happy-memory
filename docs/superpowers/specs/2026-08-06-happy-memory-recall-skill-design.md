# Diseño del skill `happy-memory-recall`

## Objetivo

Crear un skill local que enseñe a consultar memorias mediante `happy-memory` antes de diseñar, implementar, modificar, diagnosticar o tomar decisiones sobre el repositorio, y cuando un usuario o agente solicite una consulta de memoria. El skill ejecutará la consulta y los filtros elegidos por el agente principal, validará la respuesta y entregará las memorias candidatas sin resumirlas ni decidir su relevancia o aplicación.

El skill vivirá en `.agents/skills/happy-memory-recall/` y evolucionará junto con el contrato del CLI.

## Activación y límites

El skill se activará automáticamente antes de diseñar, implementar, modificar, diagnosticar o tomar decisiones sobre el repositorio. También se activará cuando el usuario o el agente solicite explícitamente recordar, recuperar contexto, consultar decisiones previas o buscar memoria.

No se activará automáticamente para operaciones mecánicas cuyo resultado no dependa del contexto acumulado, como mostrar estado, formatear archivos o ejecutar una prueba ya indicada. Una solicitud explícita siempre podrá activarlo.

Su única responsabilidad será ejecutar y presentar operaciones de recuperación. El agente principal conservará el juicio sobre los términos, filtros, continuidad de la búsqueda, relevancia de las memorias y uso del contexto. El skill no creará, actualizará, eliminará, restaurará ni resumirá memorias. Tampoco inicializará proyectos ni reparará el almacenamiento.

## Estructura

```text
.agents/skills/happy-memory-recall/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    └── search-cli.md
```

`SKILL.md` contendrá el procedimiento de recuperación: activación operativa, contrato de entrada, ejecución del CLI, validación de la respuesta, deduplicación, entrega, límite de intentos y guardrails. No contendrá heurísticas para elegir consultas o filtros ni criterios cualitativos de relevancia.

`references/search-cli.md` será la fuente técnica del contrato de lectura. Documentará la sintaxis de `search`, `tags search` y `tags list`; tipos y filtros; normalización de etiquetas; semántica textual; límites; ranking; JSON público; restricciones; errores y ejemplos. `SKILL.md` ordenará leer esta referencia antes de construir la primera consulta de cada recuperación. La lógica no se duplicará en la referencia.

`agents/openai.yaml` ofrecerá metadatos coherentes con el nombre, descripción y prompt predeterminado del skill. No se crearán scripts, assets ni documentación auxiliar.

## Responsabilidades del agente principal

El agente principal proporcionará para cada intento:

- Una consulta textual no vacía.
- Cero o un tipo.
- Cero o más etiquetas.
- Un mínimo opcional de importancia.
- Un mínimo opcional de confianza.
- Un límite opcional; si se omite, se usará 10.

Después de recibir la salida estructurada de cada intento, el agente principal decidirá si termina la recuperación o solicita otro intento. También decidirá los nuevos términos y filtros. El skill no inventará sinónimos, no elegirá un tipo o etiqueta y no calificará los resultados como relevantes, suficientes o ruidosos.

## Procedimiento de recuperación

1. Leer `references/search-cli.md` antes del primer intento y ejecutar desde el repositorio al que pertenece la tarea.
2. Recibir del agente principal la consulta y los filtros del intento.
3. Validar localmente que la consulta no esté vacía y que los valores pertenezcan a los rangos y enumeraciones documentados.
4. Construir y ejecutar `happy-memory search <query>` con los flags suministrados. Usar `--limit 10` cuando el agente principal no indique un límite.
5. Interpretar stdout como respuesta de éxito solo cuando el proceso termine con código cero y el JSON tenga `ok: true`. Interpretar stderr como respuesta de error cuando el proceso termine con código distinto de cero.
6. Convertir la respuesta al paquete YAML definido en esta especificación y devolver control al agente principal.
7. Si el agente principal solicita otro intento, repetir los pasos 2 a 6. Ejecutar como máximo tres comandos `search` por recuperación. Las consultas auxiliares `tags search` y `tags list` no consumen ese presupuesto.
8. Deduplicar memorias por ID al acumular intentos. Cuando una memoria aparezca en más de una consulta, conservar una copia y registrar todas las coincidencias.
9. No comparar ni agregar scores provenientes de consultas distintas. Cada score y posición conservarán el contexto de su intento.
10. Cerrar la recuperación con `failed` si un error impidió completarla. Si no hubo error terminal, usar `found` cuando al menos un intento devolvió una memoria y `empty` cuando todos los intentos devolvieron listas vacías.

## Contrato de filtros

### Tipo

El agente principal puede proporcionar exactamente uno de los siguientes valores para `--type`:

| Tipo | Intención |
| --- | --- |
| `fact` | Estado o comportamiento vigente |
| `decision` | Elección previa y sus motivos |
| `constraint` | Regla o límite obligatorio |
| `preference` | Convención deseada |
| `procedure` | Secuencia reutilizable |
| `lesson` | Problema conocido, hallazgo o aprendizaje |

Omitir el flag cuando el agente principal no proporcione un tipo. El CLI no admite varios tipos en una misma consulta.

### Etiquetas

Pasar cada etiqueta proporcionada por el agente principal como un flag `--tag` independiente. Varias etiquetas tienen semántica AND. Cuando el agente principal solicite descubrir vocabulario, ejecutar `tags search <query>` o `tags list`, devolver sus resultados y dejar la selección de etiquetas al agente principal.

### Importancia y confianza

Aceptar `--min-importance` y `--min-confidence` con valores enteros de 1 a 5. Omitir el flag correspondiente cuando el agente principal no proporcione un mínimo. Ambos filtros son independientes y conservan memorias cuyo valor sea mayor o igual al mínimo.

### Límite

Aceptar `--limit` con valores enteros de 1 a 100. Usar 10 cuando el agente principal no proporcione el valor.

### Capacidades inexistentes

Rechazar solicitudes de flags para fecha, versión, ID, rama, worktree, atributos, autor, máximos, OR, NOT, exclusión de etiquetas, varios tipos o memorias eliminadas porque el CLI no los implementa. Si el agente principal decide aproximar OR, ejecutar cada alternativa como un intento separado.

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
tag_lookups:
  - command: search
    query: "sqlite"
    tags:
      - name: "sqlite"
        normalized_name: "sqlite"
        description: "SQLite persistence"
        active_memory_count: 3
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

Título y contenido se preservarán sin resumen. Cada intento registrará la consulta, filtros, versión de ranking y número de resultados. `tag_lookups` registrará cada consulta auxiliar y todos los campos devueltos por el vocabulario. Cada coincidencia registrará su intento, posición y componentes de score. Las memorias de consultas distintas conservarán el orden de descubrimiento y no se reordenarán comparando scores.

`failed` prevalece si un error impide completar la recuperación e incluye un objeto `error` con `code`, `message` y `details` cuando estén disponibles. Sin error terminal, `found` significa que al menos un intento devolvió una memoria; no afirma que el agente principal la considere relevante. `empty` significa que todos los intentos ejecutados correctamente devolvieron `results: []`. En `found` y `empty`, `error` será nulo.

## Errores

- Si el ejecutable `happy-memory` no está disponible, terminar con `failed` y explicar la dependencia ausente.
- Ante `GIT_REPOSITORY_NOT_FOUND`, `PROJECT_NOT_INITIALIZED`, `STORE_BUSY` o `STORE_ERROR`, terminar con `failed`. No inicializar, mutar, reparar ni presentar el fallo como ausencia de memorias.
- Ante `VALIDATION_ERROR` causado por un comando construido por el skill, corregir la invocación una sola vez. La ejecución corregida cuenta dentro del máximo de tres comandos `search`. Si el error persiste, terminar con `failed`.
- Una ejecución exitosa con `results: []` es la única base para considerar vacío un intento.

## Validación

La implementación se validará con el validador estructural de `skill-creator` y con pruebas de uso realistas que cubran:

1. Activación automática antes de diseñar, implementar, modificar, diagnosticar o decidir sobre el repositorio.
2. Activación por una solicitud explícita de memoria.
3. Ejecución exacta de la consulta y los filtros suministrados por el agente principal.
4. Uso del límite predeterminado cuando el agente principal lo omite.
5. Consulta auxiliar del vocabulario de etiquetas sin selección automática.
6. Segundo o tercer intento solicitado por el agente principal.
7. Simulación de OR solicitada por el agente principal, deduplicación y preservación del ranking por consulta.
8. Distinción entre `found`, `empty` y errores operativos.
9. Respeto del máximo de tres búsquedas y de los límites de alcance.

Las pruebas comprobarán la transferencia del comportamiento a solicitudes nuevas sin proporcionar al evaluador las decisiones esperadas. No ejecutarán mutaciones ni operarán sobre sistemas de producción.

## Criterios de aceptación

1. La metadata activa el skill antes de diseñar, implementar, modificar, diagnosticar o decidir, y ante solicitudes explícitas, sin reclamar tareas mecánicas por defecto.
2. `SKILL.md` contiene el procedimiento de recuperación y remite al contrato técnico sin duplicarlo.
3. La referencia coincide con los comandos, filtros, semántica, salida y errores implementados por el CLI.
4. El skill ejecuta la consulta y los filtros elegidos por el agente principal sin sustituirlos por decisiones propias.
5. Ninguna recuperación supera tres ejecuciones de `search`.
6. Las consultas alternativas no mezclan scores y las memorias repetidas se deduplican por ID.
7. La entrega conserva el contenido original y toda la información necesaria para rastrear cómo se obtuvo cada memoria.
8. Un resultado vacío nunca oculta un fallo del CLI.
9. El skill no ejecuta comandos de mutación ni amplía su responsabilidad a elegir consultas, filtros, continuidad, relevancia o aplicación de las memorias.
10. La carpeta pasa la validación oficial de `skill-creator` y las pruebas realistas satisfacen el flujo aprobado.
