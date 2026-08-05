# Diseño del skill `implement-code`

## Objetivo

Crear un skill local que gobierne toda implementación, refactorización o modificación de código en `happy-memory`. El trabajo se dividirá en grupos pequeños, verificables e incrementales para limitar el alcance de cada cambio y detectar regresiones antes de continuar.

## Ubicación y estructura

El skill vivirá en `.agents/skills/implement-code/` y contendrá:

```text
.agents/skills/implement-code/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    ├── business-logic.md
    └── repositories.md
```

El contenido operativo estará escrito en inglés para mantener la convención de los skills y documentos de arquitectura existentes. `SKILL.md` será breve y no duplicará las pautas detalladas de las referencias.

## Activación

La descripción del skill deberá activarlo siempre que un agente vaya a:

- Implementar una funcionalidad.
- Refactorizar código existente.
- Corregir un defecto mediante cambios de código.
- Modificar, crear o eliminar cualquier archivo de código productivo.

## Flujo incremental

Antes de editar, el agente revisará el estado del repositorio, identificará los archivos necesarios y preservará cambios ajenos. Luego dividirá la implementación en grupos de un máximo de cinco archivos de código productivo.

Para este límite:

- Se cuentan los archivos productivos creados, modificados o eliminados en el grupo.
- No se cuentan tests, migraciones ni documentación.
- Un archivo productivo pertenece a un único grupo lógico aunque requiera correcciones durante su validación.
- Cada grupo debe representar un incremento coherente que pueda validarse antes de avanzar.

Después de implementar cada grupo, el agente ejecutará los tests relevantes y el linter configurado por el repositorio. Si una validación falla, corregirá el grupo actual y repetirá ambas comprobaciones; no iniciará el siguiente grupo mientras existan fallos atribuibles a sus cambios.

Al terminar todos los grupos, ejecutará la validación integral disponible en el repositorio y reportará los grupos implementados, los archivos productivos incluidos y los resultados de calidad.

## Referencia de lógica de negocio

`references/business-logic.md` definirá como lógica de negocio:

- Invariantes y transiciones de estado.
- Validaciones semánticas.
- Normalizaciones con significado de dominio.
- Cálculos, políticas y decisiones.
- Coordinación de casos de uso.
- Errores y códigos propios del dominio.

La referencia indicará ubicar estas responsabilidades en `internal/<domain>/`, mantenerlas independientes de frameworks y adaptadores, preferir funciones puras para reglas deterministas y usar servicios para coordinar puertos y entidades. Los puertos se declararán desde el dominio consumidor con tipos del núcleo. Las reglas, límites, errores y transiciones se cubrirán con tests unitarios.

También distinguirá la lógica de negocio de la validación sintáctica de entrada, la presentación, el acceso a datos, el mapeo de persistencia, la configuración y el cableado de dependencias.

## Referencia de repositorios

`references/repositories.md` establecerá que los repositorios:

- Implementan puertos definidos por el dominio.
- Viven en `internal/adapters/<technology>/`.
- Usan GORM para consultas, escrituras y transacciones.
- Propagan `context.Context` mediante `WithContext`.
- Mantienen modelos de persistencia separados de las entidades de dominio.
- Se limitan a persistencia, composición de consultas, mapeos y traducción de errores técnicos.
- Usan transacciones GORM para operaciones atómicas.
- Traducen `gorm.ErrRecordNotFound` y otros errores según el contrato del dominio.
- Usan Goose para cambios de esquema y nunca `AutoMigrate`.
- Se validan mediante tests de integración con SQLite.

La referencia prohibirá colocar en repositorios validaciones semánticas, normalizaciones de dominio, cálculos, políticas, transiciones de estado o cualquier otra decisión de negocio. Los filtros y ordenamientos implementarán el contrato recibido sin inventar reglas. El SQL directo solo podrá ejecutarse a través de GORM cuando sus APIs de consulta no expresen adecuadamente la operación.

## Metadata de descubrimiento

`agents/openai.yaml` incluirá únicamente `display_name`, `short_description` y `default_prompt`, derivados del contenido final del skill. No se agregarán iconos, colores ni metadata no solicitada.

## Validación del skill

La entrega se validará con el script oficial `quick_validate.py` de `skill-creator`. Además, se comprobará que:

- El frontmatter contenga únicamente `name` y `description`.
- La descripción cubra todos los disparadores relacionados con tocar código.
- `SKILL.md` enlace directamente ambas referencias y explique cuándo leer cada una.
- El límite de cinco archivos sea inequívoco.
- La validación con tests y linter ocurra después de cada grupo.
- No existan pautas contradictorias o duplicadas.
- Los archivos no contengan placeholders ni errores de formato.

## Criterios de aceptación

1. El skill se activa para cualquier implementación, refactorización o cambio de código.
2. Ningún grupo contiene más de cinco archivos de código productivo.
3. Tests, migraciones y documentación quedan fuera del conteo.
4. Cada grupo supera tests y linter antes de comenzar el siguiente.
5. La referencia de negocio identifica y ubica claramente las reglas del dominio.
6. La referencia de repositorios exige GORM y excluye toda lógica de negocio.
7. El skill permanece breve, conciso y sin duplicar sus referencias.
