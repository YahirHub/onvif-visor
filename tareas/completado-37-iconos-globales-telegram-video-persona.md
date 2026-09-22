# Tarea 37 — Iconos globales y video de persona en Telegram

## Estado
Completada y verificada.

## Implementación
- Lucide 1.47.0 cargado desde CDN y aplicado globalmente.
- data-lucide para iconos estáticos e iconMarkup()/refreshIcons() para UI dinámica.
- Los SVG manuales de iconos fueron retirados; analyticsOverlay permanece porque es un lienzo funcional.
- Telegram admite photo, video y photo_video.
- Las fotos de persona incluyen botón inline "Ver video".
- callback_data usa pv:<id> corto y persistente.
- Las acciones se guardan en SQLite ligadas al chat, cámara, grabación y offset.
- GetUpdates procesa callback_query.
- Telegram inicia/extiende la grabación necesaria para poder recuperar el pasado del evento.
- El clip se recorta con object_segments al tiempo real que la persona estuvo detectada, limitado por video_seconds.
- El modo video conserva fallback de evento ONVIF genérico cuando no se confirma una persona.
- /foto y /video permanecen compatibles.

## Seguridad
- Acción ligada al chat que recibió la alerta.
- Chat debe seguir vinculado para usar el callback.
- IDs aleatorios de 64 bits representados en 16 hex.
- Acción expira a los 7 días.
- No se persisten MP4 temporales.
- No se registran token ni RTSP con credenciales.
- Telegram mantiene límite existente de 20 MiB al subir videos.

## Pruebas
- go test ./...: OK
- go vet ./...: OK
- esbuild: OK
- git diff --check: OK
- build CGO: OK
- smoke HTTP producción: OK
- servicio OpenRC: started
- CDN Lucide 1.47.0: HTTP 200

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256: 7827addb982cd42e8b3085d4ed294234741f282a3bae9dc18228ad0ecafdcec6
- Backup: /opt/onvif-visor/backup-20260922-task37/onvif-visor
- Backup SHA256: c99f6f3199454ec2161310a988ee0727e0e25b785b27138339d1ac44614a17c2

## Nota
No existe navegador Chrome/Chromium en el host; la verificación frontend se hizo por compilación, auditoría estática, disponibilidad del CDN y smoke HTTP. La prueba visual/interactiva real queda pendiente para un navegador disponible.

README no fue modificado.
