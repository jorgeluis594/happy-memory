# Tarea 08: Validar integralmente el POC

Estado: planificada

Dependencias: tareas 01 a 07

## Contexto

Cada incremento anterior es aprobable por separado, pero el POC solo está completo si las capacidades interactúan correctamente en el almacenamiento local compartido por los worktrees de cada repositorio, a través de clones y bajo errores reales. Esta tarea constituye la puerta final de aceptación del producto.

No agrega una capacidad nueva. Consolida un conjunto repetible de escenarios de producto, corrige cualquier desviación encontrada dentro del alcance aprobado y produce evidencia de que el POC satisface su especificación completa.

## Resultado de producto

El POC puede aceptarse como un CLI local coherente: inicializa su almacenamiento, aísla proyectos, preserva memorias e historia, reutiliza etiquetas, recupera resultados deterministas y diagnostica su estado.

## Alcance

### Incluye

- Validación desde una instalación sin base previa.
- Validación de repositorio principal, worktree vinculado, repositorio movido y clone independiente.
- Recorrido de todos los comandos públicos del POC.
- Recorrido completo de creación, lectura, listado, actualización, conflicto, borrado, historial y restauración.
- Validación de etiquetas, filtros y conteos durante cambios de estado.
- Validación de búsqueda, ranking, aislamiento y exclusión de eliminadas.
- Validación de diagnóstico, migraciones, errores JSON, rollback y contención.
- Comprobación de que todas las salidas y códigos de proceso respetan el contrato público.
- Corrección de defectos que impidan cumplir criterios ya aprobados, sin ampliar el POC.
- Evidencia repetible de los resultados de aceptación.

### Fuera de alcance

- Funciones declaradas fuera del POC en la especificación.
- Cambios de pesos, tipos, comandos o modelo de datos no requeridos para cumplir el diseño aprobado.
- Pruebas de sincronización entre usuarios o máquinas.
- Medición de recuperación semántica o comparación con búsqueda vectorial.
- Nuevas operaciones administrativas destructivas.

## Criterios de aceptación de producto

1. Desde un entorno limpio, el CLI prepara su almacenamiento y permite inicializar el primer repositorio sin configuración manual de SQLite.
2. El repositorio principal, su worktree y el repositorio después de moverse conservan una identidad; un clone independiente recibe otra.
3. Todos los comandos enumerados en `docs/poc/README.md` cuentan con al menos un escenario exitoso y uno de error relevante ejecutado contra el binario real.
4. Una memoria recorre creación, consulta, listado, varias actualizaciones, conflicto de versión, borrado, consulta administrativa y restauración manteniendo snapshots completos y una versión estrictamente creciente.
5. Ningún flujo normal permite leer, listar, buscar, modificar, eliminar o restaurar una memoria perteneciente a otro proyecto.
6. La misma memoria y las mismas etiquetas pueden existir de forma independiente en dos proyectos sin colisiones entre ellos.
7. Los conteos de etiquetas y el índice FTS reflejan inmediatamente cada creación, actualización, borrado y restauración completada.
8. Una búsqueda repetida sobre el mismo estado devuelve el mismo orden, puntuaciones y `ranking_version`.
9. Todos los casos de validación, duplicado, inexistencia, conflicto, base ocupada y fallo de almacenamiento devuelven el código estable correspondiente.
10. Toda mutación forzada a fallar conserva exactamente el estado observable previo.
11. `doctor` aprueba el almacenamiento al finalizar el recorrido y detecta las inconsistencias controladas utilizadas por la validación.
12. Todas las respuestas son un único documento JSON en el canal correspondiente y todas las fechas usan UTC RFC 3339.
13. No existe una ruta de producto que acepte selección manual de proyecto, incluya eliminadas en búsquedas o exponga revisiones como estado actual.
14. La evidencia de aceptación puede volver a ejecutarse desde cero y obtiene los mismos resultados funcionales, salvo UUID y marcas de tiempo generados.
15. No queda ningún criterio de aceptación de `docs/memory-cli-product-design.md` sin un resultado aprobado o una referencia explícita a uno de los escenarios ejecutados.

## Escenario integral de validación

1. Partir de un directorio de datos vacío y crear el almacenamiento mediante el uso normal del CLI.
2. Inicializar un repositorio, un worktree vinculado y un clone independiente; mover el primero durante el recorrido.
3. Crear memorias de todos los tipos y cubrir los extremos `1` y `5` de importancia y confianza.
4. Ejercitar parches parciales, no-op, conflictos, duplicados, borrado y restauración de snapshots con etiquetas distintas.
5. Validar listados, vocabulario y búsqueda antes y después de cada cambio de estado.
6. Ejecutar accesos cruzados entre proyectos y confirmar que ninguno revela datos.
7. Provocar contención y fallos transaccionales controlados, y comprobar el estado posterior.
8. Ejecutar `doctor` y la matriz completa de comandos al final.

El POC se acepta únicamente cuando el escenario completo y todos los criterios de las tareas 01 a 07 pasan en conjunto.

## Trazabilidad

- Especificación de producto: secciones 1 a 13, incluidos todos los criterios de aceptación de la sección 12.
- Arquitectura: `docs/architecture.md` completo como restricción de la implementación validada.
