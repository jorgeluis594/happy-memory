# Tarea 01: Inicializar el CLI y su almacenamiento

Estado: planificada

Dependencias: ninguna

## Contexto

`happy-memory` necesita una base SQLite local por repositorio, ubicada en `dirname(git-common-dir)/.happy-memory/memory.db` y compartida por todos sus worktrees. Un `init` válido debe prepararla sin pasos manuales; los demás comandos nunca deben crear una base ausente.

Este incremento entrega el bootstrap del producto como parte de `happy-memory init`.

## Resultado de producto

Un repositorio puede crear y abrir de forma explícita una base SQLite privada; sus worktrees reutilizan el mismo archivo y los comandos normales exigen que ya exista.

## Alcance

### Incluye

- Resolver el `git-common-dir` absoluto y derivar la ruta local de almacenamiento.
- Crear el directorio y el archivo únicamente durante `init`.
- Restringir el acceso al directorio y al archivo al usuario actual.
- Abrir una única conexión `database/sql` y construir GORM sobre ella.
- Habilitar claves foráneas, WAL y el tiempo de espera de contención requerido por el producto.
- Exponer un cierre explícito de la conexión.
- Preparar un helper de goose que aplique migraciones recibidas como `fs.FS`, sin incluir ni ejecutar migraciones reales todavía.

### Fuera de alcance

- Consultar o modificar la antigua base global.
- Guardar un `project_id` en la configuración Git.
- Crear memorias, etiquetas o revisiones.
- Reparar automáticamente una base corrupta.
- Cifrar, sincronizar, purgar o importar la base.
- Agregar un comando público específico para inicializar el almacenamiento.
- Crear tablas del dominio, FTS5 o metadata persistente de goose durante el bootstrap.
- Definir respuestas JSON o clasificar errores como `STORE_ERROR` y `STORE_BUSY`.

## Criterios de aceptación de producto

1. Ejecutar `init` en un repositorio sin datos previos crea `.happy-memory/memory.db` sin pasos manuales.
2. El directorio y el archivo creados no conceden acceso a otros usuarios del sistema.
3. Una segunda apertura conserva los datos existentes y no vuelve a crear ni reiniciar la base.
4. La conexión tiene habilitadas las claves foráneas, WAL y un `busy_timeout` de 5000 ms.
5. GORM y las consultas directas con `database/sql` comparten la misma conexión.
6. La base recién creada no contiene tablas del dominio ni metadata de goose.
7. El helper de goose aplica correctamente migraciones proporcionadas por el llamador, pero el bootstrap normal no lo ejecuta.
8. Todos los worktrees vinculados abren el mismo archivo y un comando normal sin base devuelve `PROJECT_NOT_INITIALIZED` sin crear archivos.

## Escenario integral de validación

1. Abrir el almacenamiento en un entorno sin directorio de datos.
2. Verificar que el directorio y el archivo aparecen con permisos restringidos y que la base no contiene tablas.
3. Insertar un registro de prueba, cerrar, volver a abrir y comprobar que continúa presente.
4. Verificar las tres opciones de SQLite y el uso compartido de la conexión por GORM.
5. Ejecutar el helper contra una migración temporal y comprobar que el bootstrap normal no crea su tabla ni la metadata de goose.

El escenario se aprueba cuando la base vacía queda lista de forma automática, repetible y sin pérdida o exposición de datos, y el motor de migraciones queda disponible para la tarea siguiente.

## Trazabilidad

- Especificación de producto: secciones 2, 5, 5.7, 8.3, 8.4, 9 y 10.
- Arquitectura: secciones 1 a 5; el acceso a SQLite es un adaptador y su construcción corresponde a `internal/app`.
