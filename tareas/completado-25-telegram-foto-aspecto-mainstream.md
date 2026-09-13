# Tarea 25 — Corregir aspecto de fotos Telegram con persona

## Objetivo
Corregir definitivamente las alertas fotográficas de Telegram para que conserven la misma relación de aspecto del stream principal que se ve en el panel, sin perder el requisito de que NanoDet confirme primero una persona.

## Diagnóstico confirmado
- NanoDet detecta sobre un substream ligero (`subtype=1`) para ahorrar CPU/ancho de banda.
- En las dos cámaras reales se comprobó que el **stream principal es 16:9** mientras el substream analítico es 4:3:
  - Cocina: snapshot principal `640x360`; substream NanoDet `640x480`.
  - LC: snapshot principal `640x360`; substream NanoDet `640x480`.
- El fix anterior preservaba correctamente la geometría del substream, por eso Telegram seguía mostrando una foto distinta a la del panel aunque ya no se deformara internamente.

## Solución implementada
- NanoDet se mantiene sobre el substream para conservar el bajo consumo.
- La detección ahora guarda únicamente `DetectedAt` y confianza; ya no genera ni conserva un JPEG del substream para Telegram.
- Cuando NanoDet confirma persona, la alerta intenta extraer el frame del **MKV principal activo** usando exactamente `DetectedAt - StartedAt` como offset.
- Si el MKV activo todavía no ha volcado ese cluster o la detección es anterior al inicio del archivo, se toma inmediatamente un snapshot del **RTSP principal** de la cámara.
- `RecordingSnapshot` y `Snapshot` usan `scale=640:-2:force_original_aspect_ratio=decrease`, por lo que mantienen el aspecto del stream principal.
- Smoke real del nuevo fallback principal: Cocina `640x360`, LC `640x360`.
- Se eliminó completamente el camino `frame.JPEG` del analizador para impedir que una alerta vuelva a enviar accidentalmente el substream 4:3.

## Validación
- Tests nuevos del offset exacto de snapshot, detección anterior al inicio de grabación y estado sin grabación.
- Tests de analítica adaptados a marcador de detección sin JPEG.
- `go test ./...`: OK.
- `go vet ./...`: OK.
- Build principal CGO/ONNX: OK.
- Build estático de compatibilidad `CGO_ENABLED=0`: OK.
- `git diff --check`: OK.
- `./install.sh`: OK; OpenRC quedó `started`.
- `/api/health`: OK.
- SHA-256 del binario instalado y `dist/onvif-visor` coincide: `cec5b78215bf05b7db4d4d49240181c5f6ea52117f1c3de33ddd3ad3783826fc`.

## Resultado
La alerta de foto ya no usa la geometría 640x480 del substream NanoDet. Se envía una imagen del stream/grabación principal 640x360, manteniendo el mismo 16:9 que se ve en el panel.
