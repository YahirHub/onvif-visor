# Tarea 28 — UI de visión inteligente y timeline multiclase

## Objetivo
Hacer configurables desde la SPA las clases NanoDet, zonas, líneas y reglas, y reemplazar la pista fija Personas por un timeline dinámico por clases con miniaturas basadas en el mejor frame.

## Implementado
- Botón `AI` por cámara.
- Diálogo responsive con snapshot + overlay SVG para dibujar polígonos y líneas mediante click/touch.
- Selector buscable y agrupado de las 80 clases NanoDet/COCO; cada cámara conserva sólo las clases habilitadas.
- Zonas y líneas se pueden crear, renombrar y eliminar antes de guardar.
- Reglas se pueden crear, editar, activar/desactivar y eliminar; combinan clase, aparición/entrada/salida/cruce, zona/línea, días, horario, timezone del navegador, cooldown y Telegram.
- Guardar usa `PUT /api/analytics/config/{cameraID}` y reindexa sólo esa cámara.
- Grabaciones consume `/api/analytics/objects` y renderiza pistas dinámicas por clase en lugar de una pista fija Personas.
- Tarjetas de clips usan el segmento con mayor `best_score` para la miniatura y muestran badge de la clase.
- Eventos inteligentes muestran objeto, zona/línea, dirección, confianza y saltan al offset del objeto cuando existe.
- Resumen diario compacto muestra objetos por track, hora pico, entradas, salidas, cruces, reglas y clases principales; frontend lo cachea 15 s.
- Estados loading/error/empty y layouts móvil/tablet añadidos sin framework frontend.

## Validación
- `go test ./...`: OK.
- `go vet ./...`: OK.
- `git diff --check`: OK.
- build CGO/ONNX: OK.
- build `CGO_ENABLED=0`: OK con degradación segura.
- esbuild sobre `web/static/app.js`: OK.
- auditoría DOM: 211 IDs HTML, 200 referencias simples JS, `missing=[]`.
- smoke temporal en `127.0.0.1:18091`: UI/rutas 200, analítica ready, 80 clases desde API.
- test de migración SQLite legacy → multiclase: OK.

## Despliegue
- Servicio productivo respaldado antes de migrar: `/opt/onvif-visor/data/backup-20260913-0300-smart/analytics.db`.
- Binario desplegado en `/opt/onvif-visor/onvif-visor`; SHA-256 coincide con `dist/onvif-visor`: `7dfe48c56137f4a69981483087ea3c85b62983c94babdaf8f6e49bff64ffdfc6`.
- OpenRC: `started`.
- `/api/health`, `/api/session`, `/camaras`, `/grabaciones`, `/eventos`, `/app.js`, `/styles.css`: 200.
- APIs protegidas siguen devolviendo 401 sin sesión (`/api/cameras`, `/api/analytics/classes`).
- Esquema real verificado: `best_score`, `object_segments`, `analytics_config`, `smart_events` presentes.
- Reindex multiclase real iniciado con modelo `nanodet-plus-m_320:objects-v2`; el servicio continúa activo mientras procesa el histórico.
- Assets embebidos productivos contienen el diálogo AI, JS y estilos nuevos.

## Resultado
Tarea completada y desplegada. La validación visual final de interacción sobre el navegador del usuario queda como comprobación de uso real; no había navegador automatizado disponible en el host.
