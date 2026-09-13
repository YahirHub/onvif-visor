# Fecha
2026-09-12

# Objetivo
Cerrar la instalación automática desde código fuente, corregir la duración/final del playback, mejorar alertas Telegram con NanoDet, añadir retención/borrado de grabaciones y mantener consistencia entre clips, analítica, miniaturas y eventos.

# Decisiones tomadas
- `install.sh` es la ruta recomendada para build+install en Linux: instala dependencias, verifica Go >=1.24, resuelve ONNX Runtime, prueba, compila y ejecuta `--install`.
- ONNX Runtime se prefiere desde el gestor nativo; el fallback binario oficial sólo se usa en glibc amd64/arm64. En musl se evita mezclar binarios glibc.
- La duración exacta sólo se obtiene con `ffprobe` al abrir un clip y se cachea por path+tamaño+mtime; el listado masivo conserva la estimación barata.
- Los borrados sólo aceptan camera ID + basename validado y nunca eliminan una grabación activa.
- El borrado de un MKV debe limpiar también analítica y eventos en memoria asociados.
- `GET /api/events` filtra referencias a MKV inexistentes para reparar estados huérfanos incluso si un archivo desapareció fuera de la API.
- Miniaturas de Grabaciones y Eventos prefieren el `peak_seconds` de mayor confianza NanoDet.
- Para Telegram, el frame que se envía no debe ser el tensor 320x320 del detector. FFmpeg conserva aspecto; Go crea un letterbox 320x320 separado sólo para NanoDet.
- El selector `Medio de las alertas automáticas` no controla `/foto` ni `/video`; la duración de vídeo permanece editable en cualquier modo.
- Se eliminó el refresco de configuración Telegram acoplado al polling de cámaras porque sobrescribía la selección del usuario antes de guardarla.

# Arquitectura actual
```text
RTSP substream
  -> ffprobe ancho/alto
  -> FFmpeg 1 FPS manteniendo aspecto (ej. 320x180)
       -> JPEG notificable Telegram (misma relación)
       -> letterbox Go 320x320 -> NanoDet

MKV -> analytics.db (segmentos person + peak)
  -> Grabaciones: thumbnail ?at=peak
  -> Eventos: person_offset/person_confidence -> thumbnail ?at=peak

DELETE MKV
  -> archivo + fsync dir
  -> cache duración
  -> analytics.db
  -> CameraEvent asociados
  -> refresh frontend Eventos
```

# Librerías usadas
- Go standard library.
- FFmpeg/ffprobe externos.
- `github.com/yalue/onnxruntime_go` ya integrado.
- `modernc.org/sqlite` ya integrado.
- NanoDet-Plus-m 320 ya integrado.

# Archivos importantes modificados
- `install.sh`
- `internal/media/manager.go`, `internal/media/library.go`, tests de media.
- `internal/analytics/manager.go`, `store.go`, tests de analytics.
- `internal/server/recording_delete.go`, `handlers.go`, `server.go`, `telegram_integration.go`, tests de server.
- `web/static/index.html`, `app.js`, `styles.css`.
- `README.md`.
- `tareas/completado-21-instalador-retencion-playback-telegram.md`.

# Problemas encontrados
- La duración mostrada podía seguir avanzando después del último frame porque provenía de mtime-inicio, no del contenedor real.
- El primer enfoque de captura Telegram guardaba el frame 320x320 ya paddeado para NanoDet, provocando imagen 1:1.
- La configuración Telegram se recargaba desde el polling de cámaras cada 4 s, por lo que seleccionar `Fragmento de vídeo` podía revertirse visualmente antes de guardar.
- Los clips eliminados quitaban archivo/analítica pero dejaban eventos en memoria que apuntaban a MKV inexistentes.
- Las miniaturas se generaban al inicio/evento y no aprovechaban los offsets de persona ya indexados.

# Soluciones implementadas
- Endpoint metadata exacta + caché `ffprobe`; timer frontend termina con duración real y trata EOF final como cierre normal.
- Borrado individual y masivo con confirmación, filtros temporales, protección de grabación activa y limpieza de analytics/eventos.
- `RecordingExists` permite a `/api/events` descartar referencias huérfanas.
- `handleEvents` enriquece cada clip con el mejor segmento de persona y el frontend actualiza incrementalmente la miniatura cuando aparece dicho dato.
- Tarjetas de Grabaciones usan el frame NanoDet de mayor confianza y muestran badge de persona; fallback a thumbnail normal si ese frame falla.
- Telegram live analiza un frame reducido con aspecto real y sólo letterboxea una copia para el modelo. La foto enviada conserva proporción del substream.
- UI Telegram desacoplada del polling de cámaras y texto aclarado: medio sólo para alertas automáticas; duración independiente para vídeo.
- `install.sh` probado de forma real y utilizado para desplegar la versión final.

# Pendientes
- No hay pendiente funcional obligatorio de esta tarea.
- Falta únicamente validación visual pixel a pixel en un navegador real porque el host no dispone de navegador/CDP.

# Próximos pasos
- Probar desde un navegador real que la miniatura de persona elegida sea útil con escenas de Cocina/LC.
- Probar una alerta Telegram real y confirmar visualmente que la imagen enviada mantiene 16:9/4:3 según la cámara.
- Ajustar umbral o estrategia de selección de `peak_seconds` sólo si las detecciones reales muestran falsos positivos o encuadres poco útiles.
