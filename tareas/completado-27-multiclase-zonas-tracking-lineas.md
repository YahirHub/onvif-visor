# Tarea 27 — NanoDet multiclase, zonas y líneas de cruce

## Objetivo
Aprovechar las 80 clases que ya produce NanoDet sin aumentar la frecuencia de inferencia, permitir configurar qué objetos interesan por cámara y añadir zonas/polígonos y líneas de cruce con tracking ligero.

## Implementado
- El decoder NanoDet inspecciona las 80 salidas COCO en la **misma inferencia ONNX** y hace NMS por clase.
- Se mantienen 1 FPS, una sesión ONNX, 1 hilo intra-op + 1 inter-op; no se añadió otro modelo.
- Catálogo completo de clases con etiquetas comunes en español y grupos personas/vehículos/animales/objetos.
- `object_segments` SQLite persiste segmentos por clase, confianza, `best_score` y mejor instante.
- Migración automática copia los antiguos `person_segments` a `object_segments` cuando corresponde.
- `analytics_config` persiste por cámara clases habilitadas, zonas poligonales, líneas y reglas.
- Configuración validada con límites: 40 clases, 16 zonas, 16 líneas y 32 reglas por cámara.
- `smart_events` persiste aparición de objeto, entrada/salida de zona y cruce de línea, con track, dirección, confianza y reglas coincidentes.
- Tracker ligero por clase usa distancia de centroides + IoU, expira tracks tras 4 s y trabaja sobre coordenadas normalizadas.
- Se elimina correctamente el letterbox 320x320 al llevar bounding boxes a coordenadas de cámara.
- Zonas usan point-in-polygon; líneas detectan cambio de lado + intersección real del segmento de movimiento.
- Cruces guardan dirección `A→B` / `B→A`.
- Reglas combinan clase, tipo de evento, zona/línea, días, horario/zona horaria, cooldown y bandera Telegram.
- Análisis histórico y live usan el mismo detector multiclase y el mismo mejor-frame score.
- Al guardar configuración se reindexa sólo esa cámara.
- Las detecciones live se escriben por clase mientras existe grabación activa; zonas/líneas funcionan también sin grabación.
- API nueva:
  - `GET /api/analytics/classes`
  - `GET/PUT /api/analytics/config/{cameraID}`
  - `GET /api/analytics/objects?camera_id=...`
  - `GET /api/analytics/events?camera_id=...`
  - `GET /api/analytics/summary?...`
- `/api/analytics/persons` mantiene compatibilidad leyendo `object_segments` filtrado por `person`.
- Eventos smart live se integran al feed general de Eventos y pueden disparar Telegram si una regla lo solicita.
- Telegram smart respeta el medio configurado: foto desde MKV/RTSP principal o vídeo desde stream principal.
- Resumen diario backend cuenta objetos por clase, hora con mayor actividad, entradas/salidas de zona, cruces y reglas activadas.

## Pruebas
- Decoder multiclase prueba explícitamente `car` además de `person`.
- Tests de mejor frame, merge por clase y separación de confianza/peak.
- Tests de normalización 16:9 con letterbox, point-in-polygon, entrada/salida de zona, cruce de línea y horario de reglas.
- Tests SQLite de configuración, segmentos, eventos, borrado y fingerprint del modelo `objects-v2`.
- `go test ./internal/analytics ./internal/server` pasa durante el desarrollo del backend.

## Rendimiento
No se incrementó la frecuencia de inferencia. El costo nuevo por frame es decoder de 80 scores, NMS sólo de clases detectadas, geometría 2D y tracking de unos pocos objetos; es muy inferior al costo de la inferencia ONNX y no añade dependencias.

## Resultado
Backend de visión inteligente multiclase, zonas, líneas, tracking, reglas y resumen listo. La siguiente tarea conecta configuración visual y timeline multiclase en la SPA.
