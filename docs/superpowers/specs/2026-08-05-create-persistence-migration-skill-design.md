# Diseño del skill `create-persistence-migration`

## Objetivo

Crear un skill local y versionado que materialice una migración SQLite con Goose a partir de una especificación completa proporcionada por el agente que lo invoca.

El skill no descubre ni diseña cambios de dominio. El agente llamador conserva la responsabilidad de definir la estructura requerida y de actualizar modelos, repositorios, consultas y pruebas funcionales.

## Ubicación y estructura

El skill vivirá en `.agents/skills/create-persistence-migration/` y contendrá:

- `SKILL.md`, con el procedimiento específico del repositorio.
- `agents/openai.yaml`, con metadata de descubrimiento.

Todo el contenido del skill estará escrito en inglés. No incluirá scripts, plantillas ni referencias separadas.

## Contrato de entrada

El agente llamador deberá proporcionar la información necesaria para crear la migración:

- Nombre o propósito de la migración.
- Estado de esquema que debe producir `Up`.
- Comportamiento requerido para `Down`, o una indicación explícita de que no existe una reversión segura.
- Transformaciones de datos requeridas, si corresponde.
- Restricciones, claves foráneas, índices y demás detalles relevantes.

El skill no buscará requisitos adicionales en tareas POC, modelos o repositorios. Solo pedirá aclaraciones si la especificación recibida es insuficiente o contradictoria para producir SQL seguro.

## Flujo operativo

Al activarse, el skill indicará al agente que:

1. Localice el directorio incorporado por el `embed.FS` y determine la siguiente versión secuencial disponible.
2. Cree primero un archivo SQL Goose con cinco dígitos y un nombre descriptivo.
3. Escriba las secciones `-- +goose Up` y `-- +goose Down` usando exclusivamente la especificación recibida.
4. Use la transacción predeterminada de Goose y permita `NO TRANSACTION` solo cuando sea técnicamente necesario.
5. Verifique que el archivo esté incluido por el `embed.FS` y sea consumible por `sqlite.Migrate`.
6. Ejecute una validación dirigida que confirme que Goose puede analizar y aplicar la migración.
7. Entregue la ruta creada y el resultado de las validaciones.

Si se trata de la primera migración y aún no existe infraestructura embebida, el skill podrá crear el `embed.FS` mínimo y conectarlo con el helper existente. No deberá implementar otros cambios de persistencia.

## Límites de responsabilidad

El skill no deberá:

- Analizar tareas POC para descubrir el cambio solicitado.
- Diseñar el esquema a partir de requisitos de producto.
- Actualizar modelos GORM, repositorios, consultas o DTO internos.
- Crear pruebas generales de instalaciones nuevas o actualizaciones entre versiones.
- Ejecutar el conjunto completo de verificaciones del producto, salvo que el agente llamador lo solicite expresamente.
- Usar `AutoMigrate`.
- Modificar, renumerar o eliminar migraciones históricas que puedan haberse aplicado.
- Truncar o reemplazar la base de datos para facilitar la migración.
- Implementar una pérdida de datos no autorizada.

## Validación propia del skill

La validación se limita al artefacto creado y su integración inmediata:

- Nombre y numeración secuencial correctos.
- Presencia y orden de las anotaciones Goose requeridas.
- SQL aceptado por SQLite y Goose.
- Inclusión en el sistema de archivos embebido.
- Aplicación mediante `sqlite.Migrate` sobre una base temporal cuando la infraestructura disponible lo permita.
- Ausencia de errores de formato en los archivos modificados.

Las pruebas de dominio, compatibilidad de repositorios y comportamiento integral pertenecen al agente llamador.

## Criterios de aceptación

1. La descripción activa el skill cuando un agente solicita crear una migración SQLite con Goose.
2. El skill trata la especificación recibida como fuente del cambio y no inicia una fase de descubrimiento de requisitos.
3. El primer resultado material es el archivo de migración versionado.
4. El procedimiento valida el artefacto y su integración con Goose sin asumir responsabilidad por el resto de la capa de persistencia.
5. El contenido del skill y su metadata están completamente en inglés.
6. El skill conserva el historial de migraciones y evita pérdidas de datos no autorizadas.
