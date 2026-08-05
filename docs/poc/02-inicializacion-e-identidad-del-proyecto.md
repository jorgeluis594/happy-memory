# Tarea 02: Inicializar e identificar proyectos

Estado: planificada

Dependencias: Tarea 01

## Contexto

Las memorias deben pertenecer al repositorio desde el cual trabaja el agente, sin depender de rutas, ramas o remotos. La identidad se guarda en la configuración común de Git para que todos los worktrees vinculados compartan memoria, mientras que un clone independiente recibe otra identidad.

Este incremento conecta un repositorio Git con el almacenamiento global ya preparado y permite inspeccionar el catálogo local de proyectos.

## Resultado de producto

El usuario puede inicializar un repositorio, consultar su identidad estable y distinguirlo de otros proyectos sin seleccionar manualmente un `project_id`.

## Alcance

### Incluye

- `happy-memory init [--name <name>]`.
- Detección del repositorio Git asociado al directorio de trabajo.
- Resolución de la configuración común de Git.
- Generación de un UUID y persistencia bajo la clave local específica del producto.
- Creación o reconciliación del registro en `projects`.
- Nombre explícito con `--name` o inferido del directorio principal del repositorio.
- Actualización informativa de `last_known_git_common_dir` cuando corresponda.
- Comportamiento idempotente de `init`.
- Recuperación del registro SQLite cuando Git conserva el `project_id` pero la fila de proyecto no existe.
- `happy-memory project show` dentro del proyecto activo.
- `happy-memory projects list`, incluso fuera de un repositorio.
- Errores JSON estables cuando el repositorio no existe o aún no está inicializado.

### Fuera de alcance

- Crear, buscar o modificar memorias.
- Derivar la identidad desde la ruta, el remote, el commit o la rama.
- Copiar identidad hacia clones independientes.
- Aceptar `--project-id` en operaciones normales.
- Sincronizar proyectos entre máquinas o usuarios.

## Criterios de aceptación de producto

1. `happy-memory init` ejecutado dentro de un repositorio Git crea un único `project_id`, lo guarda en la configuración común y registra el proyecto en SQLite.
2. La respuesta exitosa es JSON, contiene la identidad y el nombre efectivo del proyecto, se escribe en `stdout` y termina con código cero.
3. Repetir `init` en el mismo repositorio conserva el `project_id` y no crea un segundo proyecto.
4. Cuando se omite `--name`, el proyecto recibe un nombre inferido del directorio principal; cuando se proporciona, usa el nombre indicado sin alterar la identidad.
5. Dos worktrees vinculados muestran el mismo `project_id`.
6. Un clone independiente inicializado muestra un `project_id` diferente, aunque tenga el mismo remote y contenido.
7. Mover el repositorio y volver a consultarlo conserva el `project_id`; la ruta conocida puede actualizarse solo como información.
8. Si Git conserva el identificador pero falta el registro de SQLite, `init` recrea el registro con el mismo UUID.
9. `project show` devuelve únicamente el proyecto resuelto desde el repositorio actual.
10. `projects list` puede ejecutarse fuera de un repositorio y muestra cada proyecto global una sola vez.
11. Ejecutar `init` fuera de Git falla con una respuesta JSON y no crea un proyecto huérfano.
12. Ejecutar una operación normal dentro de un repositorio no inicializado devuelve `PROJECT_NOT_INITIALIZED`.
13. Ninguna operación normal expone un flag que permita seleccionar manualmente otro `project_id`.

## Escenario integral de validación

1. Inicializar un repositorio principal con un nombre explícito.
2. Crear un worktree vinculado y confirmar que `project show` devuelve la misma identidad.
3. Mover el repositorio principal y confirmar que mantiene esa identidad.
4. Crear un clone independiente, inicializarlo y confirmar que recibe otra identidad.
5. Ejecutar `projects list` fuera de Git y comprobar que aparecen exactamente los dos proyectos.
6. Eliminar únicamente la fila SQLite del proyecto principal, volver a ejecutar `init` y confirmar que se reconcilia con el identificador conservado en Git.

El escenario se aprueba cuando la identidad sigue las reglas de Git y no las rutas o remotos.

## Trazabilidad

- Especificación de producto: secciones 2, 3, 5.2, 6.5, 8 y criterios de aceptación de identidad.
- Arquitectura: los casos de uso pertenecen al módulo de proyecto; Git y SQLite se mantienen detrás de adaptadores separados.
