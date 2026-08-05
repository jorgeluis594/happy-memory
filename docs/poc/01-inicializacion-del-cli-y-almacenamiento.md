# Tarea 01: Inicializar el CLI y su almacenamiento

Estado: planificada

Dependencias: ninguna

## Contexto

`happy-memory` necesita una única base SQLite global, ubicada fuera de los repositorios, antes de poder registrar proyectos o memorias. Una instalación nueva no debe exigir que el usuario cree archivos o ejecute SQL manualmente. Una instalación existente tampoco debe perder información cuando el binario vuelve a ejecutarse o incorpora una migración compatible.

Este incremento entrega el bootstrap del producto. No es la inicialización de un repositorio Git y no introduce un comando público adicional: las operaciones que necesitan almacenamiento aseguran primero que este se encuentre listo.

## Resultado de producto

Una instalación nueva de `happy-memory` puede crear y abrir automáticamente una base SQLite local, privada y vacía. El motor de migraciones queda preparado para los incrementos posteriores, sin ejecutarse todavía durante el bootstrap.

## Alcance

### Incluye

- Resolver el directorio estándar de datos de aplicaciones del sistema operativo para el usuario actual.
- Crear el directorio y el archivo de base de datos cuando no existen.
- Restringir el acceso al directorio y al archivo al usuario actual.
- Abrir una única conexión `database/sql` y construir GORM sobre ella.
- Habilitar claves foráneas, WAL y el tiempo de espera de contención requerido por el producto.
- Exponer un cierre explícito de la conexión.
- Preparar un helper de goose que aplique migraciones recibidas como `fs.FS`, sin incluir ni ejecutar migraciones reales todavía.

### Fuera de alcance

- Crear o identificar un proyecto Git.
- Guardar un `project_id` en la configuración Git.
- Crear memorias, etiquetas o revisiones.
- Reparar automáticamente una base corrupta.
- Cifrar, sincronizar, purgar o importar la base.
- Agregar un comando público específico para inicializar el almacenamiento.
- Crear tablas del dominio, FTS5 o metadata persistente de goose durante el bootstrap.
- Definir respuestas JSON o clasificar errores como `STORE_ERROR` y `STORE_BUSY`.

## Criterios de aceptación de producto

1. Abrir el almacenamiento en un entorno de usuario sin datos previos crea el directorio de aplicación y una base SQLite vacía sin pasos manuales.
2. El directorio y el archivo creados no conceden acceso a otros usuarios del sistema.
3. Una segunda apertura conserva los datos existentes y no vuelve a crear ni reiniciar la base.
4. La conexión tiene habilitadas las claves foráneas, WAL y un `busy_timeout` de 5000 ms.
5. GORM y las consultas directas con `database/sql` comparten la misma conexión.
6. La base recién creada no contiene tablas del dominio ni metadata de goose.
7. El helper de goose aplica correctamente migraciones proporcionadas por el llamador, pero el bootstrap normal no lo ejecuta.
8. La preparación del almacenamiento funciona sin depender del directorio de trabajo ni de que este pertenezca a un repositorio Git.

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
