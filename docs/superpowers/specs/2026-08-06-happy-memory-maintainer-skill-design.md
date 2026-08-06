# Diseño del skill `happy-memory-maintainer`

## Resultado

Crear un skill autónomo que mantenga el conocimiento durable del proyecto mediante el CLI `happy-memory`. El skill podrá crear, actualizar, dividir y eliminar memorias, además de mantener sus tags como keywords. Se activará tanto por una solicitud explícita como durante cualquier trabajo del repositorio cuando alcance un punto natural de consolidación.

El skill no convertirá la memoria en un registro de actividad. Cada memoria representará una idea pequeña, atómica, completa y reutilizable.

## Alcance

El incremento incluye:

- El skill `.agents/skills/happy-memory-maintainer/`.
- Detección de información durable.
- Selección autónoma entre crear, actualizar, dividir, eliminar o no mutar.
- Creación de varias memorias atómicas en un mismo punto de consolidación.
- Mantenimiento posterior a refactors y cambios de lógica.
- Escalas descriptivas de importancia y confiabilidad entre `1` y `5`.
- Selección y mantenimiento de tags como keywords.
- Escritura optimizada para la recuperación determinista de `happy-memory-recall`.
- Uso seguro del contrato JSON, control optimista de versiones y errores estables del CLI.
- Divulgación progresiva mediante una referencia directa por caso.

Quedan fuera de alcance:

- Cambiar el código del CLI.
- Implementar `--limit` para `tags search`; el skill asumirá que estará disponible.
- Renombrar, fusionar o eliminar tags directamente.
- Inicializar o reparar automáticamente un proyecto o su almacenamiento.
- Borrado físico de memorias o revisiones.
- Operaciones batch o una transacción que abarque varias memorias.
- Scripts y assets propios del skill.
- Ejemplos de memorias o de las escalas, porque dependen del contexto de cada proyecto.

## Fuentes de verdad

El diseño se apoya en el comportamiento actual del dominio y del CLI, y en el contrato de lectura documentado por `happy-memory-recall`.

Las propiedades relevantes son:

- El CLI no decide semánticamente qué crear, modificar o eliminar; esa decisión pertenece al consumidor.
- Los tipos válidos son `fact`, `decision`, `constraint`, `preference`, `procedure` y `lesson`.
- Importancia y confiabilidad son enteros obligatorios e independientes entre `1` y `5`.
- `create` recibe un estado completo por JSON en `stdin`.
- `update` recibe un patch, exige `expected-version` y reemplaza el conjunto completo cuando `tags` está presente.
- `delete` es lógico, exige `expected-version` y preserva historial.
- No existe una mutación batch.
- La búsqueda textual solo indexa el título y el contenido activos.
- Los tags sirven como filtros, no como texto indexado.
- La búsqueda separa términos por espacios, exige todos los términos con semántica AND y no expande sinónimos.
- BM25 pondera el título con `5` y el contenido con `1`.
- El ranking combina `0.70` de texto, `0.20` de importancia y `0.10` de confiabilidad.
- `search` no devuelve la versión necesaria para mutar; debe seguirse con `get`.

## Artefactos

El skill tendrá esta estructura:

```text
.agents/skills/happy-memory-maintainer/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    ├── create-memories.md
    ├── update-memories.md
    ├── delete-memories.md
    ├── maintain-after-refactor.md
    ├── score-memories.md
    ├── maintain-tags.md
    ├── write-retrievable-memories.md
    └── cli-contract.md
```

`SKILL.md` será un enrutador compacto. Definirá los invariantes compartidos y enlazará directamente cada referencia. No duplicará los procedimientos detallados.

`agents/openai.yaml` expondrá únicamente `display_name`, `short_description` y un `default_prompt` que mencione `$happy-memory-maintainer`.

## Activación y puntos de consolidación

El skill se activará:

- Explícitamente, cuando el usuario solicite agregar, corregir, dividir, eliminar o mantener memorias o tags.
- Implícitamente, cuando cualquier trabajo produzca información durable del proyecto.

No mutará después de cada observación intermedia. Acumulará candidatos durante el trabajo y actuará en puntos naturales de consolidación, entre ellos la confirmación de una decisión, restricción, preferencia, procedimiento, hecho o lección; la corrección de una creencia previa; un refactor o cambio de lógica; y el cierre de una tarea con conocimiento reutilizable.

Un punto puede generar varias memorias. El skill separará primero las ideas y ejecutará después una mutación independiente por memoria.

## Información durable

`SKILL.md` definirá como durable la información que probablemente seguirá siendo útil después de la tarea o conversación actual y evitará que un agente futuro tenga que redescubrirla.

La información durable:

- Está vinculada al proyecto, producto, arquitectura, flujo de trabajo o preferencias aplicables.
- Puede influir en decisiones, implementaciones o diagnósticos futuros.
- Conserva utilidad al cambiar de tarea, sesión o agente.
- Expresa al menos una idea completa y comprensible fuera de su contexto inmediato.
- No representa únicamente el estado momentáneo del trabajo.

La durabilidad y la certeza son dimensiones diferentes. La incertidumbre se expresará mediante confiabilidad, mientras que el impacto futuro se expresará mediante importancia.

## Atomicidad

Una memoria será atómica cuando contenga una sola idea reutilizable que pueda evaluarse, actualizarse o invalidarse sin depender de ideas independientes. Puede contener el contexto mínimo, una razón o varios pasos cuando sean inseparables de esa única idea.

El skill dividirá una candidata cuando:

- Sus afirmaciones puedan cambiar por separado.
- Mezcle tipos de memoria distintos.
- Sus partes requieran diferente importancia, confiabilidad o tags.
- Una parte pueda eliminarse sin invalidar las demás.

La reducción del número de comandos nunca justificará combinar ideas independientes.

## Enrutamiento por casos

`SKILL.md` contendrá esta tabla conceptual de enrutamiento:

| Caso detectado | Referencia obligatoria |
| --- | --- |
| Nueva información durable | `references/create-memories.md` |
| Corrección, evolución o división | `references/update-memories.md` |
| Retiro de una memoria activa | `references/delete-memories.md` |
| Refactor o cambio de lógica | `references/maintain-after-refactor.md` |
| Asignación o revisión de puntuaciones | `references/score-memories.md` |
| Selección o mantenimiento de keywords | `references/maintain-tags.md` |
| Redacción o reescritura recuperable | `references/write-retrievable-memories.md` |
| Ejecución de cualquier comando | `references/cli-contract.md` |

El agente cargará solo las referencias aplicables. Una creación normal requerirá creación, puntuación, tags, escritura recuperable y contrato CLI. Un refactor comenzará con su referencia específica y solo añadirá las referencias de las mutaciones realmente detectadas.

## Crear memorias

`create-memories.md` ordenará crear cuando la información sea durable, completa, atómica y no esté representada por una memoria activa.

El procedimiento será:

1. Separar las ideas independientes.
2. Buscar memorias relacionadas mediante consultas textuales específicas y acotadas.
3. Comparar las candidatas por identidad conceptual, no solo por igualdad exacta.
4. Elegir el tipo según la naturaleza de cada idea.
5. Redactar título y contenido recuperables.
6. Asignar importancia y confiabilidad.
7. Resolver tags.
8. Ejecutar y verificar una creación por memoria.

Si una memoria activa ya representa adecuadamente la idea, el resultado será no hacer nada. Si representa la misma idea con un estado distinto, se enrutará a actualización.

## Actualizar y dividir memorias

`update-memories.md` ordenará actualizar cuando una memoria existente represente la misma idea, pero requiera corregir o ajustar título, contenido, tipo, puntuaciones o tags. No se ampliará con ideas independientes.

La división será una operación coordinada de actualización:

1. Identificar todas las ideas independientes de la memoria original.
2. Reutilizar la memoria original para una de ellas cuando sea coherente.
3. Crear las demás memorias atómicas.
4. Asignar individualmente tipo, importancia, confiabilidad y tags.
5. Verificar que toda información durable haya quedado persistida.
6. Eliminar la memoria original solo cuando no pueda conservarse como una unidad resultante.

El skill obtendrá siempre el estado y la versión actuales mediante `get`. Enviará un patch mínimo. Si el patch incluye `tags`, enviará el conjunto completo que debe permanecer asociado.

## Eliminar memorias

`delete-memories.md` definirá la eliminación como el retiro de una memoria del conocimiento activo. Se aplicará cuando su presencia ya no aporte información futura y no exista un estado vigente de la misma idea que deba expresarse mediante actualización.

Los criterios descriptivos incluirán:

- Duplicación conceptual cuya información durable ya esté preservada en otra memoria activa.
- Ruido o información almacenada por error.
- Información perteneciente a otro alcance que no corresponde al proyecto actual.
- Residuo de una división cuya información quedó completamente preservada en unidades atómicas verificadas.
- Información invalidada cuando conservar explícitamente su obsolescencia tampoco tenga valor durable.

No se eliminará por antigüedad, poco uso, importancia baja, confiabilidad baja, falta de relación con la tarea actual ni por la mera existencia de un refactor. Si la obsolescencia es información durable, se actualizará el contenido para expresarla.

Antes de eliminar, el skill obtendrá la versión actual, comprobará que no se perderá información durable y recordará que el borrado del CLI es lógico.

## Mantenimiento después de refactors

`maintain-after-refactor.md` convertirá cualquier refactor o cambio de lógica en un punto obligatorio de revisión.

El procedimiento será:

1. Identificar el resultado durable del cambio.
2. Buscar memorias relacionadas con el código modificado, con atención especial a la lógica de negocio afectada.
3. Comparar cada memoria con el comportamiento vigente.
4. Actualizar contenido, tipo, importancia, confiabilidad o tags cuando hayan cambiado.
5. Marcar explícitamente como obsoleto el conocimiento que ya no sea vigente pero cuya obsolescencia siga siendo durable.
6. Dejar intactas las memorias que continúen siendo correctas.

Un refactor no reducirá automáticamente la confiabilidad ni causará una eliminación. La nueva afirmación se puntuará según su propia evidencia y utilidad futura.

## Importancia y confiabilidad

`score-memories.md` contendrá las escalas completas, sin ejemplos.

### Importancia

Mide el impacto futuro de olvidar la información:

1. **Mínima:** utilidad muy localizada; olvidarla tendría consecuencias insignificantes.
2. **Baja:** puede ahorrar trabajo en situaciones limitadas; es fácil de redescubrir.
3. **Media:** influye de forma apreciable en trabajo futuro; olvidarla puede causar retrabajo o decisiones deficientes.
4. **Alta:** afecta sustancialmente la corrección, el diseño o la operación; olvidarla implica riesgo o costo significativo.
5. **Crítica:** condiciona el funcionamiento fundamental del proyecto; olvidarla puede provocar consecuencias graves o incumplir una regla esencial.

### Confiabilidad

Mide la solidez y vigencia de la afirmación almacenada:

1. **Especulativa:** existe una posibilidad razonable, pero falta evidencia o hay contradicciones importantes.
2. **Débil:** dispone de evidencia parcial, indirecta o todavía no confirmada.
3. **Moderada:** la evidencia es creíble, aunque incompleta, limitada por contexto o pendiente de una comprobación relevante.
4. **Alta:** está respaldada directamente por fuentes confiables y solo queda incertidumbre menor.
5. **Confirmada:** está respaldada por evidencia autoritativa, verificable y vigente, sin conflicto material conocido.

Las escalas se aplicarán así:

- La importancia se basa en las consecuencias de olvidar, no en la cantidad de evidencia.
- La confiabilidad se basa en evidencia y vigencia, no en impacto.
- La ambigüedad reduce confiabilidad, pero no necesariamente importancia.
- Partes que requieren puntuaciones diferentes deben separarse.
- Una actualización solo recalibra cuando cambia la evidencia, vigencia o impacto.
- Una memoria marcada como obsoleta puntúa la nueva afirmación sobre su obsolescencia.
- No se usarán promedios ni fórmulas automáticas.

## Tags como keywords

`maintain-tags.md` definirá los tags como keywords estructuradas para agrupar y filtrar temas relacionados.

El skill:

- No ejecutará `tags list`.
- Formulará una consulta específica para cada keyword candidata.
- Ejecutará directamente `happy-memory tags search "<query>" --limit 10`.
- Comparará nombre normalizado, descripción y uso activo antes de seleccionar.
- Reutilizará un nombre canónico antes de crear una variante.
- Seleccionará el conjunto mínimo suficiente, sin un número fijo.
- Evitará sinónimos redundantes, términos demasiado generales y detalles transitorios.
- Mantendrá separados el tipo, las puntuaciones y los tags.
- Conservará asociaciones todavía pertinentes cuando actualice.
- Retirará asociaciones que ya no describan el contenido vigente.
- Recalculará tags individualmente al dividir.

Una keyword nueva podrá incluir una descripción breve solo cuando su alcance sea claro. El skill tendrá presente que la primera descripción persiste y que el CLI no permite mantener directamente el registro del tag mediante rename, merge o delete.

## Escritura para recuperación

`write-retrievable-memories.md` trasladará las restricciones de `happy-memory-recall` a reglas de redacción.

Al crear, reescribir o dividir, el skill:

- Usará en el título la terminología canónica y discriminativa del proyecto.
- Mantendrá el título corto y centrado en la idea principal.
- Redactará contenido autocontenido que no dependa de la conversación original.
- Incluirá naturalmente en título o contenido los términos que probablemente se usarán para recuperar la idea.
- Incluirá términos alternativos únicamente cuando pertenezcan al lenguaje real del proyecto.
- Evitará relleno de keywords y repeticiones artificiales.
- Asignará tipo y puntuaciones por significado, no para manipular el ranking.
- Expresará claramente vigencia u obsolescencia cuando corresponda.
- Colocará en título o contenido los conceptos esenciales aunque también existan como tags.

La atomicidad y la terminología consistente son parte de la recuperabilidad, no optimizaciones opcionales.

## Contrato de ejecución

`cli-contract.md` será la única referencia técnica detallada para comandos, JSON, tipos, versiones y errores.

El flujo compartido será:

1. Ejecutar desde el repositorio asociado al proyecto.
2. Buscar memorias relacionadas con consultas específicas y límites acotados.
3. Usar `get` antes de actualizar o eliminar.
4. Enviar documentos y patches como JSON estricto por `stdin`.
5. Ejecutar una mutación por memoria.
6. Considerar éxito solo un código de salida cero y JSON con `ok: true`.
7. Verificar cada resultado antes de continuar con una operación dependiente.

El contrato documentará:

```text
happy-memory create --input -
happy-memory get <memory-id> [--include-deleted]
happy-memory search "<query>" [filters] --limit 10
happy-memory update <memory-id> --expected-version <n> --input -
happy-memory delete <memory-id> --expected-version <n>
happy-memory tags search "<query>" --limit 10
```

No se utilizará `tags list`.

## Concurrencia y errores

Ante errores:

- `VERSION_CONFLICT`: recuperar el estado actual, reevaluar la mutación y reintentar una sola vez si continúa siendo correcta.
- `DUPLICATE_MEMORY`: inspeccionar la memoria existente y decidir entre actualizarla o no mutar; no crear una variante para eludir el error.
- `VALIDATION_ERROR`: corregir una vez una invocación generada cuando el contrato permita identificar el error.
- `GIT_REPOSITORY_NOT_FOUND`, `PROJECT_NOT_INITIALIZED`, `STORE_BUSY` o `STORE_ERROR`: detener las operaciones dependientes sin inicializar, reparar ni convertir el fallo en ausencia de memoria.
- Ejecutable ausente: detener el mantenimiento y reportar la ausencia.

Cuando un punto produzca varias memorias, un fallo no revertirá mutaciones anteriores. El skill podrá continuar con operaciones independientes, pero detendrá toda secuencia que dependa del paso fallido. Una división creará y verificará primero las nuevas unidades antes de modificar o eliminar la original.

## Divulgación progresiva

Las referencias no se enlazarán entre sí como requisito oculto. Todas estarán expuestas directamente desde `SKILL.md`, con una frase que indique exactamente cuándo leerlas.

El contenido se distribuirá sin duplicación:

- `SKILL.md`: activación, definición durable, atomicidad, autonomía, puntos de consolidación y tabla de enrutamiento.
- `create-memories.md`: decisión y procedimiento de creación múltiple.
- `update-memories.md`: identidad conceptual, patches y división.
- `delete-memories.md`: criterios descriptivos y protección contra pérdida.
- `maintain-after-refactor.md`: revisión de lógica modificada y obsolescencia.
- `score-memories.md`: escalas y reglas de asignación.
- `maintain-tags.md`: selección, consulta acotada y asociaciones.
- `write-retrievable-memories.md`: redacción compatible con recall.
- `cli-contract.md`: sintaxis, payloads, respuestas, versiones y errores.

## Validación

La implementación se validará mediante:

1. Inicialización con `skill-creator/scripts/init_skill.py`.
2. Validación estructural con `skill-creator/scripts/quick_validate.py`.
3. Comprobación de que todas las referencias de `SKILL.md` existen y son directas.
4. Búsqueda de placeholders, contradicciones, ejemplos no permitidos y contenido duplicado.
5. Revisión de consistencia entre `SKILL.md`, `agents/openai.yaml` y las referencias.
6. Confirmación de que cada caso tiene condiciones de entrada, procedimiento, condiciones de salida y errores aplicables.
7. Confirmación de que el skill usa exclusivamente `tags search "<query>" --limit 10` para descubrir tags.
8. Confirmación de que divisiones y eliminaciones no pierden información durable.
9. Confirmación de que la redacción respeta la búsqueda literal, el peso del título y el rol no textual de los tags.
10. Ejecución de `git diff --check` sobre los artefactos.

La validación no mutará memorias reales. El comando futuro `tags search --limit` se revisará de forma estática en esta iteración.

## Criterios de aceptación

El diseño se considera implementado cuando:

- El skill se activa explícita e implícitamente en los contextos aprobados.
- Define información durable y atomicidad sin ejemplos específicos de proyecto.
- Puede enrutar sin ambigüedad cada situación al caso correspondiente.
- Permite crear varias memorias pequeñas en un mismo punto.
- Trata la división como actualización coordinada.
- Revisa memorias relacionadas después de refactors sin eliminarlas por el mero cambio de código.
- Describe con precisión cuándo actualizar conocimiento obsoleto y cuándo eliminar.
- Asigna importancia y confiabilidad con escalas independientes de `1` a `5`.
- Mantiene tags mediante búsquedas filtradas de máximo 10 resultados.
- Redacta memorias recuperables por el comportamiento determinista de recall.
- Carga solo las referencias necesarias para el caso activo.
- Maneja versiones y errores sin sobrescribir cambios concurrentes ni ocultar fallos.
- Pasa la validación estructural y de consistencia sin placeholders.
