# Tarea 26 — Mejor frame inteligente

## Objetivo
Implementar primero una selección de miniatura más útil que no dependa sólo de la confianza NanoDet. El mejor frame debe favorecer objetos grandes, centrados y nítidos sin añadir otro modelo ni otra inferencia.

## Implementado
- El detector ahora devuelve bounding boxes/clase en lugar de sólo un booleano de persona.
- `scoreDetectionFrame` combina confianza (55%), tamaño (20%), centralidad (15%) y nitidez aproximada (10%).
- La nitidez se estima con gradientes de luminancia muestreados dentro del bounding box; no requiere modelo adicional.
- `person_segments` migra con columna `best_score` y conserva `max_confidence` por separado.
- `peak_seconds` representa el mejor frame del segmento, no necesariamente el de mayor confianza.
- El análisis offline y live calculan/persisten el score sin cambiar el muestreo de 1 FPS.
- Tests deterministas verifican que un objeto grande/centrado/nítido gane frente a uno pequeño en borde con mayor confianza y que confianza/peak queden desacoplados.

## Validación
- `gofmt`: OK.
- `go test ./internal/analytics`: OK.

## Resultado
Mejor frame inteligente completado como primer bloque, listo para reutilizarse por la analítica multiclase.
