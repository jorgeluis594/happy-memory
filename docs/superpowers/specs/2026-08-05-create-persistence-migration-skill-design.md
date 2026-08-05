# Diseño del skill `create-persistence-migration`

## Objetivo

Crear un skill local y versionado que guíe de principio a fin cada cambio en la estructura de datos de `happy-memory`. El skill se activará al solicitar una nueva migración o al modificar tablas, columnas, índices, restricciones, relaciones o FTS5 en la capa de persistencia.

## Ubicación y estructura

El skill vivirá en `.agents/skills/create-persistence-migration/` y contendrá:

- `SKILL.md`, con el flujo operativo y las reglas específicas del repositorio.
- `agents/openai.yaml`, con el nombre visible, la descripción breve y el prompt predeterminado.

No incluirá scripts, plantillas ni referencias separadas inicialmente. El contenido de cada migración depende del cambio solicitado y todavía no existe el directorio definitivo de migraciones embebidas; automatizar su generación ahora añadiría una convención prematura.

## Flujo operativo

Al activarse, el skill indicará al agente que:

1. Inspeccione las migraciones, el esquema, el helper `sqlite.Migrate`, los modelos, repositorios, consultas, pruebas y documentación afectados.
2. Consulte la documentación vigente de Goose mediante Context7 antes de depender de detalles de su API o anotaciones SQL.
3. Defina el cambio de esquema, sus invariantes, la compatibilidad con datos existentes y una estrategia de reversión segura.
4. Cree una migración SQL Goose con numeración secuencial y nombre descriptivo, sin reescribir migraciones ya incorporadas.
5. Incluya secciones `-- +goose Up` y `-- +goose Down`; use transacciones por defecto y permita excepciones solo cuando SQLite o la operación lo exijan y queden justificadas.
6. Actualice todos los modelos, repositorios, consultas y adaptadores afectados, sin usar GORM `AutoMigrate`.
7. Para la primera migración del producto, incorpore el `embed.FS` y conecte la ejecución con `sqlite.Migrate`. Para migraciones posteriores, reutilice esa infraestructura.
8. Añada pruebas que cubran la creación desde cero, la actualización desde la versión anterior preservando datos, las restricciones e índices relevantes, la ejecución repetida y el rollback cuando sea seguro.
9. Ejecute las comprobaciones dirigidas y generales del repositorio, incluida la compilación sin CGO.
10. Entregue un resumen del cambio, las decisiones de compatibilidad, las verificaciones ejecutadas y cualquier limitación de reversión.

## Reglas de seguridad y consistencia

- No usar `AutoMigrate` como sustituto de una migración SQL versionada.
- No editar una migración existente que pueda haberse aplicado; agregar una nueva migración secuencial.
- No eliminar ni transformar datos sin una estrategia explícita de preservación o una autorización clara para una operación irreversible.
- No asumir que `Down` es seguro: si revertir perdería datos, detenerse y pedir una decisión antes de implementar esa reversión.
- Mantener la propiedad de la conexión en el llamador: `sqlite.Migrate` recibe `*sql.DB` y no debe cerrarlo.
- Mantener el esquema portable con `CGO_ENABLED=0` y compatible con el driver SQLite puro Go usado por el proyecto.
- Preservar cambios no relacionados presentes en el árbol de trabajo.

## Validación

Cada cambio de esquema deberá incluir, según corresponda:

- Una prueba que aplique todas las migraciones sobre una base vacía.
- Una prueba de actualización desde la versión inmediatamente anterior con datos representativos.
- Verificaciones del esquema y de la conservación de datos.
- Una segunda ejecución que confirme que no se repiten migraciones aplicadas.
- Una prueba de `Down` y posterior `Up` cuando la reversión sea segura y forme parte del cambio.
- Pruebas de modelos, repositorios o consultas modificados.
- `go test -race ./...`.
- `make check`.
- `CGO_ENABLED=0 go build ./cmd/happy-memory`.

## Criterios de aceptación del skill

1. Su descripción lo activa tanto para migraciones explícitas como para cualquier cambio estructural de persistencia.
2. El procedimiento obliga a revisar el estado real del esquema antes de editarlo.
3. El procedimiento produce migraciones SQL Goose secuenciales, revisables y compatibles con la infraestructura del repositorio.
4. Los cambios de código y las pruebas permanecen sincronizados con el esquema.
5. Las validaciones cubren instalaciones nuevas y actualizaciones con datos existentes.
6. El skill evita `AutoMigrate`, la modificación retrospectiva de migraciones y las pérdidas de datos no autorizadas.

## Fuera de alcance

- Crear ahora la primera migración de producto.
- Elegir por adelantado el esquema de `projects`, memorias, revisiones, etiquetas o FTS5.
- Añadir un generador automático de archivos mientras no exista una convención de directorio consolidada.
- Cambiar el helper `sqlite.Migrate` o ejecutar migraciones durante el bootstrap como parte de la creación del skill.
