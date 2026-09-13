# Contexto 14 — Visión inteligente multiclase, zonas y reglas

## Decisión principal
Se mantuvo NanoDet-Plus-m 320 a 1 FPS y una sola sesión ONNX. No se añadió otro modelo ni se aumentó la frecuencia de inferencia. La misma salida NanoDet ahora decodifica las 80 clases COCO y sólo calcula métricas adicionales para las clases habilitadas por cámara.

## Mejor frame
Cada detección conserva bounding box y clase. El mejor frame de un tramo usa un score ligero: 55% confianza, 20% tamaño relativo, 15% centralidad y 10% nitidez aproximada por gradientes de luminancia. `max_confidence` permanece separado de `best_score`; `peak_seconds` apunta al mejor frame. Esto alimenta miniaturas, saltos de timeline y eventos.

## Persistencia
`data/analytics.db` se migra de forma incremental:
- `person_segments.best_score` para compatibilidad con el esquema anterior.
- `object_segments` para tramos por clase.
- `analytics_config` para clases habilitadas, zonas, líneas y reglas por cámara.
- `smart_events` para `object`, `zone_enter`, `zone_exit` y `line_cross`.

Si `object_segments` está vacío durante la primera migración, los tramos históricos `person` se copian automáticamente. La versión de fingerprint del análisis cambió a `nanodet-plus-m_320:objects-v2`, por lo que el histórico se vuelve a indexar una sola vez.

## Tracking y geometría
El tracking live es efímero y ligero: misma clase + distancia de centroides + IoU, expiración tras 4 s. No es reconocimiento ni reidentificación. Los bounding boxes del tensor 320x320 se convierten a coordenadas normalizadas eliminando el letterbox. Las zonas usan point-in-polygon. Las líneas requieren cambio de lado e intersección del segmento de movimiento; se guarda dirección `A→B` o `B→A`.

## Reglas
Cada cámara puede combinar clase, evento, zona/línea, días, horario, zona horaria, cooldown y Telegram. Límites actuales: 40 clases habilitadas, 16 zonas, 16 líneas y 32 reglas por cámara. Las reglas Telegram reutilizan el medio foto/vídeo de la integración existente y no envían el histórico reindexado: sólo eventos smart live.

## Frontend
La tarjeta de cámara incorpora botón `AI`. El diálogo permite:
- buscar/seleccionar clases NanoDet;
- dibujar polígonos y líneas con click/touch sobre un snapshot;
- renombrar/eliminar geometrías;
- crear, editar, habilitar/deshabilitar y eliminar reglas;
- guardar y reindexar sólo esa cámara.

Grabaciones reemplaza la pista fija Personas por pistas dinámicas por clase, usa el mejor frame para la miniatura de cada clip y muestra un resumen diario de objetos por track, hora pico, entradas, salidas, cruces y reglas activadas. Eventos entiende los campos smart y salta al offset de objeto cuando existe.

## Rendimiento
El decoder añade ~80 comparaciones por punto NanoDet, pero no otra inferencia. NMS se hace por clase sólo sobre candidatos que superan el umbral. El score de nitidez se calcula sólo para clases habilitadas; el detector de persona legacy mantiene compatibilidad con las alertas ONVIF existentes. El resumen diario se cachea 15 s en frontend.

## Validación previa al despliegue
- `go test ./...`: OK.
- `go vet ./...`: OK.
- `git diff --check`: OK.
- build CGO/ONNX: OK.
- build `CGO_ENABLED=0`: OK con analítica degradada.
- esbuild valida `web/static/app.js`: OK.
- auditoría DOM: 211 IDs HTML, 200 referencias simples JS, `missing=[]`.
- smoke temporal en `127.0.0.1:18091`: UI/rutas 200, analítica ready y `/api/analytics/classes` devuelve 80 clases.
- test de migración crea un `analytics.db` legacy sin `best_score` y verifica migración a `object_segments`: OK.

## Despliegue
- Antes de migrar se detuvo OpenRC y se respaldó `analytics.db` en `/opt/onvif-visor/data/backup-20260913-0300-smart/`.
- `dist/onvif-visor` se instaló en `/opt/onvif-visor/onvif-visor`; ambos archivos tienen SHA-256 `7dfe48c56137f4a69981483087ea3c85b62983c94babdaf8f6e49bff64ffdfc6`.
- OpenRC volvió a estado `started`; health/session y rutas SPA responden 200, APIs protegidas continúan en 401 sin cookie.
- La base real contiene `person_segments.best_score`, `object_segments`, `analytics_config` y `smart_events`. En la primera verificación había 170 filas migradas en `object_segments` y ya se estaban generando eventos smart live.
- El reindexado histórico `nanodet-plus-m_320:objects-v2` arrancó correctamente (estado `analyzing` + clips ya `complete`) sin detener el servicio.
- Los assets servidos desde producción contienen el diálogo AI, JS y CSS nuevos.

La interacción visual final debe revisarse en el navegador real del usuario porque el host no dispone de navegador automatizado.
