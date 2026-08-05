# Inicialización e identidad de proyectos

Fecha: 2026-08-05

## Objetivo

Implementar la Tarea 02 para que `happy-memory` asigne una identidad UUID estable a cada repositorio Git, la comparta entre sus worktrees vinculados y mantenga un catálogo global de proyectos en SQLite. La identidad no dependerá de rutas, ramas, commits ni remotos, y un clone independiente tendrá una identidad distinta.

## Alcance

Este incremento incorpora los comandos `init [--name <name>]`, `project show` y `projects list`; el módulo de dominio `project`; adaptadores independientes para Git, SQLite y Cobra; una migración Goose embebida; y la composición de la aplicación en `internal/app`.

No se crearán memorias, no se sincronizarán proyectos entre instalaciones, no se expondrá `--project-id` y no se coordinarán dos procesos que ejecuten `init` simultáneamente. La idempotencia garantizada es secuencial.

## Arquitectura

La implementación seguirá la arquitectura modular hexagonal existente:

- `internal/project` contendrá `Project`, errores de dominio, puertos y los casos de uso `Initialize`, `ShowCurrent` y `List`.
- `internal/adapters/git` implementará el puerto de identidad del repositorio mediante el ejecutable `git`.
- `internal/adapters/sqlite` contendrá la migración embebida y el repositorio de proyectos basado en `database/sql`.
- `internal/adapters/cobra` validará entradas, ejecutará casos de uso y serializará el contrato JSON.
- `internal/app` abrirá SQLite, aplicará migraciones, construirá dependencias, ejecutará el comando raíz y cerrará recursos.
- `cmd/happy-memory` se limitará a llamar a la aplicación y devolver su código de salida.

Los puertos pertenecerán al módulo consumidor. Ningún tipo de Cobra, Git, Goose o SQLite cruzará hacia el dominio.

## Modelo y puertos

`Project` tendrá los campos `ID`, `Name`, `LastKnownGitCommonDir`, `CreatedAt` y `UpdatedAt`. El identificador será una UUID canónica y las fechas estarán en UTC.

El dominio consumirá puertos con estas responsabilidades:

- Identidad Git: resolver el contexto común del repositorio, leer la identidad existente, escribir una identidad nueva e inferir el nombre del repositorio principal.
- Repositorio de proyectos: buscar por UUID, crear o reconciliar, renombrar explícitamente, actualizar condicionalmente la ruta conocida y listar.
- Reloj: proporcionar el instante actual para timestamps deterministas en pruebas.
- Generador de UUID: producir una identidad nueva cuando Git aún no contiene una.

Los contratos concretos se mantendrán mínimos y orientados a los casos de uso, evitando un repositorio CRUD genérico.

## Identidad Git

El adaptador invocará `git` sin incorporar una librería adicional. Resolverá `git rev-parse --git-common-dir`, normalizará el resultado a una ruta absoluta y operará exclusivamente sobre `<git-common-dir>/config` mediante `git config --file`. No leerá ni escribirá la configuración global o la configuración específica de un worktree.

La clave será `happy-memory.project-id`. Al leerla, el adaptador exigirá exactamente un valor y que ese valor sea una UUID canónica. Una clave ausente representa un repositorio no inicializado. Valores múltiples, vacíos o inválidos representan corrupción y producirán `STORE_ERROR`; nunca serán sobrescritos automáticamente.

Al crear una identidad, el adaptador escribirá primero la UUID en la configuración común. Los worktrees vinculados compartirán así la misma clave. Un clone, con otro directorio Git común, empezará sin clave y recibirá otra UUID.

El nombre inferido se obtendrá del repositorio principal, no del worktree actual. En un repositorio no bare será el nombre del directorio raíz del repositorio principal. En un repositorio bare será el nombre del directorio común sin el sufijo final `.git`. Un nombre inferido o explícito se recortará y deberá quedar no vacío.

## Persistencia SQLite

La migración embebida `00001_create_projects.sql` creará:

```sql
projects(
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  last_known_git_common_dir TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)
```

Cada campo tendrá una restricción que impida texto vacío. `Down` ejecutará `DROP TABLE projects`. La migración será repetible a través de Goose y se aplicará durante la composición normal de la aplicación.

El repositorio usará `database/sql`. La reconciliación será por UUID y no creará duplicados. Al crear una fila fijará `created_at` y `updated_at` al instante actual. Una reconciliación sin cambios preservará ambos timestamps. Un renombrado explícito o cambio de ruta preservará `created_at` y actualizará `updated_at`. La lista se ordenará de forma determinista por `name` y luego `id`.

## Casos de uso

### `Initialize`

1. Resolver el repositorio Git y su directorio común absoluto.
2. Leer `happy-memory.project-id`.
3. Si no existe, generar una UUID y escribirla primero en Git. Si la escritura falla, no crear ninguna fila SQLite.
4. Buscar la fila SQLite por UUID.
5. Si se proporcionó `--name`, validarlo y usarlo para crear o renombrar el proyecto.
6. Si no se proporcionó nombre y existe una fila, conservar su nombre.
7. Si no se proporcionó nombre y falta la fila, inferirlo desde Git. Esto cubre tanto una inicialización nueva como la recuperación de una fila eliminada.
8. Crear o reconciliar la fila, actualizando la ruta conocida solo cuando cambió.

Repetir `init` secuencialmente sin cambios devolverá la misma identidad y el mismo registro. Si Git conserva la UUID pero la fila fue eliminada, `init` recreará la fila con esa UUID.

### `ShowCurrent`

Resolverá el repositorio y leerá su UUID. Si falta la clave o la fila SQLite devolverá `PROJECT_NOT_INITIALIZED`; no reparará la fila. Si la fila existe y la ruta común cambió, actualizará únicamente esa información y `updated_at` antes de devolver el proyecto.

### `List`

Listará el catálogo SQLite sin consultar Git, por lo que funcionará desde cualquier directorio. Cada UUID aparecerá una sola vez y el orden será por nombre e ID.

## CLI y contrato público

Los éxitos se escribirán solo en `stdout`, terminarán con salto de línea y usarán exit code `0`.

`init` y `project show` devolverán:

```json
{"ok":true,"data":{"id":"6f9619ff-8b86-d011-b42d-00c04fc964ff","name":"example","last_known_git_common_dir":"/repos/example/.git","created_at":"2026-08-05T12:00:00Z","updated_at":"2026-08-05T12:00:00Z"}}
```

`projects list` devolverá:

```json
{"ok":true,"data":{"projects":[]}}
```

Las fechas se serializarán en UTC con RFC 3339. Los errores se escribirán solo en `stderr`, sin usage ni texto adicional de Cobra, y usarán exit code `1`:

```json
{"ok":false,"error":{"code":"PROJECT_NOT_INITIALIZED","message":"project is not initialized","details":{}}}
```

Los códigos públicos serán:

- `GIT_REPOSITORY_NOT_FOUND`: el directorio actual no pertenece a un repositorio Git.
- `PROJECT_NOT_INITIALIZED`: falta la identidad Git o la fila SQLite requerida por `project show`.
- `VALIDATION_ERROR`: nombre vacío, comando desconocido, argumentos posicionales o flags inválidos.
- `STORE_ERROR`: error inesperado al ejecutar Git, identidad persistida corrupta o error SQLite/migración.

El adaptador Cobra desactivará el texto de usage y normalizará también los errores de parseo y comandos inválidos. Ningún comando registrará `--project-id`.

## Composición y ciclo de vida

La aplicación recibirá los streams y argumentos necesarios para poder probarse sin lanzar un subproceso. En cada ejecución abrirá la base global, aplicará las migraciones embebidas, conectará reloj, UUID, Git, repositorio y comandos, y cerrará la base antes de devolver el exit code. Los fallos de apertura, migración o cierre se presentarán como `STORE_ERROR`; un fallo de cierre no reemplazará un error principal ya emitido.

## Pruebas

Las pruebas unitarias del dominio usarán puertos falsos y cubrirán UUID nueva y reutilizada, idempotencia, nombre inferido, renombrado, recuperación de fila, cambio de ruta y propagación de errores.

Las pruebas SQLite cubrirán migración embebida y repetible, esquema y restricciones, reconciliación sin duplicados, preservación y actualización de timestamps, listado determinista y fila ausente.

Las pruebas Git aislarán la configuración global y crearán repositorios temporales para verificar: fuera de Git, repositorio principal, worktree vinculado, clone independiente, movimiento, repositorio bare y `extensions.worktreeConfig` activo. También verificarán identidades ausentes, múltiples y no canónicas.

Las pruebas CLI comprobarán JSON exacto, separación de `stdout` y `stderr`, exit codes, códigos estables, argumentos y flags inválidos, y ausencia de `--project-id`.

Una prueba end-to-end reproducirá el escenario de aceptación: repositorio principal con nombre explícito, worktree con la misma identidad, repositorio movido, clone con identidad distinta, lista global con dos proyectos, eliminación manual de una fila y recuperación mediante `init`.

La aceptación final ejecutará `go test -race -cover ./...` y `make check`.

## Decisiones deliberadas

- Se usa el ejecutable Git porque expresa con precisión la semántica de `git-common-dir` y evita una dependencia nueva.
- La UUID se escribe antes en Git para que un fallo posterior de SQLite sea recuperable mediante otro `init` con la misma identidad.
- `project show` no repara filas ausentes para mantener una operación de lectura predecible; la reparación queda explícitamente en `init`.
- La ruta se conserva solo como información y nunca participa en la identidad.
- No se implementa locking entre procesos en este incremento; dos `init` concurrentes quedan fuera del contrato.
