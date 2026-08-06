# Diagnóstico y confiabilidad operativa

## Objetivo

Incorporar un diagnóstico global, no destructivo y ejecutable fuera de un repositorio Git, y reforzar el almacenamiento SQLite frente a esquemas incompatibles, contención y mutaciones parciales.

## Arquitectura

`internal/diagnostic` define el informe público y coordina una secuencia fija de comprobaciones mediante un puerto `StoreChecker`. El adaptador SQLite abre para diagnóstico únicamente archivos existentes en modo lectura y comprueba por separado que el archivo sea escribible, sin crearlo, migrarlo ni repararlo. La CLI expone `happy-memory doctor` antes de resolver Git o componer servicios dependientes de proyecto.

Los comandos normales conservan migraciones automáticas únicamente cuando el historial aplicado es vacío o un prefijo exacto y conocido de las migraciones embebidas. Versiones desconocidas, huecos o metadatos inválidos se rechazan antes de ejecutar Goose.

## Comprobaciones

El informe siempre contiene, en este orden, `database_access`, `schema_compatibility`, `sqlite_integrity`, `foreign_keys`, `fts5` y `fts_index_consistency`. Cada elemento incluye `name`, `ok`, `message` y `details`. Una comprobación dependiente se reporta como fallida o bloqueada con detalles seguros; nunca se omite.

La consistencia FTS compara todas las memorias activas de la base con `memory_fts` y detecta ausencias, duplicados, sobrantes y diferencias de título o contenido.

## Concurrencia y atomicidad

Las conexiones normales usan WAL, claves foráneas y `busy_timeout=100ms`. Cada transacción de escritura se reintenta completa hasta tres intentos, con pausas de 50 y 100 ms que respetan el contexto. `SQLITE_BUSY` y `SQLITE_LOCKED`, incluidas variantes extendidas, se reconocen por el código tipado del driver. Agotado el presupuesto, el error público es `STORE_BUSY` con mensaje `storage is busy`.

`Create`, `Mutate`, `Reconcile` y `UpdatePath` son unidades atómicas: ningún estado, revisión, etiqueta o entrada FTS puede quedar parcialmente aplicado.

## Contrato CLI

Todos los resultados son un único documento JSON. Los éxitos se escriben en `stdout`; los errores en `stderr`. Cobra no imprime ayuda ni texto adicional.

`doctor` saludable devuelve exit 0 y `{ "ok": true, "data": { "healthy": true, "database_path": "...", "checks": [...] } }`. Un diagnóstico no saludable devuelve exit distinto de cero, código `STORE_ERROR`, mensaje `storage diagnostics failed` y el informe completo en `error.details`.

Los demás fallos usan un contrato interno `PublicError` con código, mensaje estable y detalles seguros. Errores internos no exponen causas ni trazas.

## Verificación

Las pruebas cubren diagnóstico sano y no saludable, ausencia de base sin creación, esquemas atrasados, desconocidos o con huecos, integridad, claves foráneas, FTS5 y todas las divergencias del índice. También cubren lectura en WAL durante escritura, reintento exitoso, contención persistente, rollback total e invariantes de canal, exit code y JSON. La validación final es `go test -race ./...` y `make check`.

