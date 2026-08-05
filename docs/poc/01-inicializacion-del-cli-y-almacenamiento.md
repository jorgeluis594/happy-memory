# Tarea 01: Inicializar el CLI y su almacenamiento

Estado: planificada

Dependencias: ninguna

## Contexto

`happy-memory` necesita una única base SQLite global, ubicada fuera de los repositorios, antes de poder registrar proyectos o memorias. Una instalación nueva no debe exigir que el usuario cree archivos o ejecute SQL manualmente. Una instalación existente tampoco debe perder información cuando el binario vuelve a ejecutarse o incorpora una migración compatible.

Este incremento entrega el bootstrap del producto. No es la inicialización de un repositorio Git y no introduce un comando público adicional: las operaciones que necesitan almacenamiento aseguran primero que este se encuentre listo.

## Resultado de producto

Una instalación nueva de `happy-memory` puede preparar automáticamente un almacenamiento local, privado y compatible. Ejecuciones posteriores reutilizan los datos existentes y aplican únicamente los cambios de esquema requeridos.

## Alcance

### Incluye

- Resolver el directorio estándar de datos de aplicaciones del sistema operativo para el usuario actual.
- Crear el directorio y el archivo de base de datos cuando no existen.
- Restringir el acceso al directorio y al archivo al usuario actual.
- Cargar el esquema versionado completo requerido por el POC, incluido FTS5.
- Registrar y comprobar la versión del esquema.
- Aplicar migraciones embebidas, ordenadas y transaccionales cuando la base está desactualizada.
- Habilitar claves foráneas, WAL y el tiempo de espera de contención requerido por el producto.
- Ejecutar el bootstrap de forma idempotente antes de cualquier comando que necesite almacenamiento.
- Responder mediante el contrato JSON estable si el almacenamiento no puede prepararse.

### Fuera de alcance

- Crear o identificar un proyecto Git.
- Guardar un `project_id` en la configuración Git.
- Crear memorias, etiquetas o revisiones.
- Reparar automáticamente una base corrupta.
- Cifrar, sincronizar, purgar o importar la base.
- Agregar un comando público específico para inicializar el almacenamiento.

## Criterios de aceptación de producto

1. En un entorno de usuario sin datos previos, la primera operación que requiere almacenamiento crea el directorio de aplicación, la base SQLite y el esquema completo sin pasos manuales.
2. El directorio y el archivo creados no conceden acceso a otros usuarios del sistema.
3. Una segunda ejecución conserva los datos existentes y no vuelve a crear ni reiniciar la base.
4. Si la base tiene una versión anterior compatible, la siguiente ejecución aplica todas las migraciones pendientes en orden y deja disponible la versión esperada por el binario.
5. Si una migración falla, no queda un esquema parcialmente actualizado y la operación solicitada no continúa.
6. Si la base tiene una versión incompatible o más reciente que la soportada, el comando falla de forma segura sin modificarla.
7. Después del bootstrap están habilitadas las claves foráneas, el modo WAL y FTS5.
8. Si el directorio o la base no son accesibles, el proceso termina con código distinto de cero y emite en `stderr` un error JSON con `ok: false` y un código estable de almacenamiento.
9. Una inicialización exitosa no escribe mensajes no JSON en `stdout` ni en `stderr`.
10. La preparación del almacenamiento funciona sin depender del directorio de trabajo ni de que este pertenezca a un repositorio Git.

## Escenario integral de validación

1. Ejecutar una operación administrativa en un entorno sin directorio de datos.
2. Verificar que el almacenamiento global y el esquema aparecen con permisos restringidos.
3. Crear un registro de prueba mediante una operación soportada por una tarea posterior.
4. Volver a ejecutar el CLI y comprobar que el registro continúa presente.
5. Simular una base con una migración compatible pendiente y confirmar que se actualiza sin perder el registro.
6. Simular un fallo durante una migración y confirmar que la versión y el esquema anteriores permanecen íntegros.

El escenario se aprueba cuando el almacenamiento queda listo de forma automática, repetible y sin pérdida o exposición de datos.

## Trazabilidad

- Especificación de producto: secciones 2, 5, 5.7, 8.3, 8.4, 9 y 10.
- Arquitectura: secciones 1 a 5; el acceso a SQLite es un adaptador y su construcción corresponde a `internal/app`.
