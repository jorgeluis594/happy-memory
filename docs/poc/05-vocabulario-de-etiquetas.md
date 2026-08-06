# Tarea 05: Gestionar el vocabulario de etiquetas

Estado: planificada

Dependencias: tareas 01 a 04

## Contexto

Las etiquetas expresan los temas de las memorias y forman un vocabulario reutilizable dentro de cada proyecto. Sin herramientas de descubrimiento, los agentes pueden crear variaciones innecesarias o desconocer las descripciones ya disponibles.

Este incremento completa el comportamiento de etiquetas iniciado al crear y actualizar memorias, y ofrece comandos para consultar el vocabulario efectivo del proyecto.

## Resultado de producto

El usuario puede descubrir, buscar y reutilizar etiquetas normalizadas, con descripciones y conteos de uso activos, sin mezclar vocabularios entre proyectos.

## Alcance

### Incluye

- Normalización determinista del nombre de una etiqueta.
- Reutilización de etiquetas equivalentes dentro del proyecto activo.
- Descripción opcional al crear por primera vez una etiqueta.
- Conservación de la descripción existente cuando otra memoria vuelve a usar la etiqueta.
- Aislamiento de etiquetas y asociaciones por `project_id`.
- `happy-memory tags list`.
- `happy-memory tags search <query> [--limit <n>]` sobre nombre y descripción, con límite predeterminado `10` y rango válido de `1` a `100`.
- Conteo de memorias activas asociadas a cada etiqueta.
- Orden de búsqueda por conteo activo y nombre normalizado.
- Integración con los filtros de etiquetas de `list` y `search`.

### Fuera de alcance

- Taxonomía cerrada o jerárquica.
- Sinónimos, fusión o renombrado de etiquetas.
- Eliminación explícita de etiquetas sin uso.
- Búsqueda semántica de etiquetas.
- Compartir automáticamente un vocabulario entre proyectos.

## Criterios de aceptación de producto

1. La normalización convierte el nombre a minúsculas, recorta espacios externos, reemplaza secuencias internas de espacios por guiones y colapsa guiones repetidos.
2. Un nombre cuyo resultado normalizado queda vacío produce `VALIDATION_ERROR`.
3. Variaciones normalizables de un nombre reutilizan una sola etiqueta dentro del mismo proyecto.
4. Dos proyectos pueden tener etiquetas con el mismo nombre normalizado sin compartir ID, descripción, conteos ni asociaciones.
5. Cuando una memoria introduce una etiqueta nueva con descripción, esa descripción queda disponible en las consultas del vocabulario.
6. Cuando una memoria reutiliza una etiqueta existente y proporciona otra descripción, la descripción existente no cambia.
7. `tags list` devuelve como mínimo ID, nombre canónico, descripción opcional y número de memorias activas de cada etiqueta del proyecto.
8. El borrado lógico de una memoria reduce sus conteos activos; restaurarla vuelve a incrementarlos.
9. `tags search` realiza una coincidencia de texto sin distinguir mayúsculas sobre nombre y descripción.
10. `tags search` ordena primero por cantidad de memorias activas descendente y después por nombre normalizado ascendente.
11. `tags search` devuelve como máximo `10` resultados por defecto, acepta `--limit` entre `1` y `100` y aplica el límite después del ordenamiento; `tags list` permanece ilimitado.
12. Una asociación que intentase unir una memoria y una etiqueta de proyectos distintos es rechazada y no deja cambios parciales.
13. Cuando un filtro repite `--tag`, solamente coinciden memorias que poseen todas las etiquetas solicitadas.
14. Las respuestas nunca incluyen etiquetas de otro proyecto, aunque el texto buscado coincida exactamente.

## Escenario integral de validación

1. Crear en un proyecto una memoria con `Git Operations` y descripción.
2. Crear otra con ` git--operations ` y una descripción distinta.
3. Confirmar mediante `tags list` que existe una sola etiqueta, conserva la primera descripción y tiene conteo activo `2`.
4. Buscar una palabra presente solo en la descripción y confirmar que la etiqueta aparece.
5. Eliminar una memoria y comprobar que el conteo baja a `1`; restaurarla y comprobar que vuelve a `2`.
6. Crear la misma etiqueta en otro proyecto y comprobar que los listados y conteos permanecen independientes.

El escenario se aprueba cuando el vocabulario es reutilizable, consistente y completamente aislado por proyecto.

## Trazabilidad

- Especificación de producto: secciones 5.6, 6.1, 6.2, 6.4, 6.6, 7.1, 8.1, 11 y criterios de aceptación de etiquetas.
- Arquitectura: las reglas de normalización y asociación pertenecen al dominio; la base impide relaciones entre proyectos.
