# Tarea 06: Buscar con ranking explicable

Estado: planificada

Dependencias: tareas 01 a 05

## Contexto

Persistir memorias solo es útil si un agente puede recuperar las más relevantes de forma predecible. El POC delega la intención semántica al consumidor y ofrece una búsqueda textual determinista que combina FTS5 con importancia y confianza.

Este incremento entrega la recuperación textual completa, incluidos filtros, ranking estable y explicación de cada puntuación.

## Resultado de producto

El usuario puede buscar texto dentro de las memorias activas del proyecto y entender por qué cada resultado ocupa su posición.

## Alcance

### Incluye

- `happy-memory search <query> [filters]` con consulta obligatoria.
- Búsqueda FTS5 únicamente sobre título y contenido actuales.
- Tratamiento de la consulta como texto plano y escape de sintaxis especial.
- Exclusión permanente de memorias eliminadas y revisiones históricas.
- Filtros por tipo, etiquetas, importancia mínima y confianza mínima.
- Semántica AND para etiquetas repetidas.
- Límite solicitado con máximo de `100`.
- Pesos FTS5 de título `5` y contenido `1`.
- Ventana fija de hasta `100` candidatos filtrados.
- Normalización determinista de BM25, importancia y confianza.
- Fórmula de ranking versión 1 y desempates estables.
- Componentes de puntuación y `ranking_version` en la respuesta.

### Fuera de alcance

- Búsqueda sin consulta o recuperación inicial automática.
- Embeddings, vectores o interpretación semántica.
- Búsqueda sobre atributos, etiquetas, revisiones o memorias eliminadas.
- Pesos, perfiles o modos configurables por el consumidor.
- Persistencia de las puntuaciones calculadas.

## Criterios de aceptación de producto

1. Una consulta devuelve solamente memorias activas del proyecto actual cuyo título o contenido coinciden mediante FTS5.
2. Una palabra presente únicamente en etiquetas, atributos o revisiones no produce una coincidencia.
3. Una memoria eliminada nunca aparece, aunque se solicite inclusión de eliminadas en otros comandos.
4. Texto que contiene operadores o caracteres especiales de FTS5 se trata como texto plano, no altera la expresión de consulta ni permite inyectar sintaxis.
5. Los filtros por tipo, importancia mínima y confianza mínima se aplican antes de calcular la ventana de ranking.
6. Cada `--tag` adicional usa semántica AND y las variaciones normalizables resuelven la etiqueta canónica del proyecto.
7. Un límite mayor que `100` o con un formato inválido produce `VALIDATION_ERROR`; nunca se evalúan más de `100` candidatos para normalizar el texto.
8. La importancia se normaliza como `(importance - 1) / 4` y la confianza como `(confidence - 1) / 4`.
9. El texto se normaliza por min-max entre el mejor y el peor BM25 de la ventana; con un solo candidato o valores BM25 iguales, todos reciben `text_score` de `1`.
10. La puntuación final usa `0.70 × text + 0.20 × importance + 0.10 × confidence`.
11. Los empates se resuelven por puntuación final, importancia, confianza y fecha de actualización descendentes, seguidos por ID ascendente.
12. Cada resultado incluye las puntuaciones final, textual, de importancia y de confianza; la respuesta identifica `ranking_version: 1`.
13. El mismo estado de base, consulta, filtros y versión de algoritmo produce el mismo orden y las mismas puntuaciones.
14. La búsqueda nunca devuelve ni permite inferir memorias de otro proyecto.
15. La puntuación se calcula para la consulta y no se guarda en la memoria ni en sus revisiones.

## Escenario integral de validación

1. Crear varias memorias que compartan términos entre título y contenido, con distintas importancias y confianzas.
2. Incluir una memoria cuyo término coincidente exista solo en una etiqueta y otra memoria eliminada.
3. Buscar el término con filtros y confirmar que solo entran candidatos activos que coinciden en título o contenido.
4. Calcular de manera independiente los componentes y comprobar el orden, las puntuaciones y los desempates devueltos.
5. Repetir la consulta y confirmar un resultado idéntico.
6. Ejecutarla desde otro proyecto y confirmar que no aparece ningún candidato del primero.
7. Probar caracteres especiales de FTS5 y comprobar una respuesta segura y válida.

El escenario se aprueba cuando el ranking puede reproducirse a partir de la respuesta y del estado visible de las memorias.

## Trazabilidad

- Especificación de producto: secciones 5.7, 6.3, 7, 8, 9, 11 y criterios de aceptación de búsqueda.
- Arquitectura: el algoritmo y sus reglas pertenecen al dominio de búsqueda; FTS5 es un detalle del adaptador de almacenamiento.
