# Fecha
2026-09-13

# Objetivo
Corregir definitivamente la relación de aspecto de las alertas fotográficas de Telegram cuando NanoDet detecta personas.

# Problema real
La alerta estaba enviando el JPEG generado desde el mismo substream ligero usado por NanoDet. Aunque ese JPEG ya respetaba la geometría del substream, el substream de las cámaras reales no tiene la misma relación de aspecto que el stream principal mostrado en el panel.

Verificación real sin exponer credenciales:
- Cocina: stream principal/snapshot `640x360` (16:9), substream analítico `640x480` (4:3).
- LC: stream principal/snapshot `640x360` (16:9), substream analítico `640x480` (4:3).

Esto explica la captura reportada por el usuario: el vídeo/stream principal se ve correcto, mientras la foto automática de persona conservaba la geometría 4:3 del substream.

# Decisión técnica
- No cambiar NanoDet al stream principal para evitar aumentar decodificación, ancho de banda y CPU.
- Separar completamente **detección** de **imagen enviada**.
- NanoDet conserva únicamente instante (`DetectedAt`) y confianza de la detección live.
- La foto final se obtiene del stream/grabación principal.

# Flujo implementado
```text
RTSP principal subtype=0 (16:9)
        |                        \
        |                         \-> grabador MKV principal
        |
        +-> analyticsStreamURI -> subtype=1 (4:3) -> 1 FPS -> NanoDet
                                                 |
                                                 -> DetectedAt + confidence
                                                          |
                                                          v
Telegram foto:
  1. RecordingSnapshot(MKV principal, DetectedAt-StartedAt)
  2. si el MKV activo aún no permite leer ese instante:
     Snapshot(RTSP principal)
```

# Cambios importantes
- `internal/analytics/manager.go`
  - `PersonFrame` ya no contiene `JPEG`.
  - `WaitForPersonFrame` espera únicamente un marcador reciente de detección.
  - eliminado `encodeBGRJPEG` y toda codificación de imagen del substream para Telegram.
  - `rememberPersonFrame` conserva sólo hora/confianza.
- `internal/server/telegram_integration.go`
  - nuevo `personRecordingSnapshotTarget` calcula nombre y offset del MKV principal.
  - nuevo `telegramPersonPhoto` intenta primero snapshot del MKV principal en el instante exacto y luego RTSP principal como fallback.
  - la alerta ya no puede enviar `frame.JPEG` del substream.
- `README.md` actualizado para describir el flujo correcto.
- Tests actualizados/agregados en analítica y Telegram.

# Validación
- Comparación real previa al fix: ambas cámaras `primary_snapshot=640x360`, `analytics_substream=640x480`.
- Smoke real del nuevo `telegramPersonPhoto` usando fallback al stream principal:
  - Cocina: `640x360`.
  - LC: `640x360`.
- `go test ./...`: OK.
- `go vet ./...`: OK.
- Build CGO/ONNX: OK.
- Build `CGO_ENABLED=0`: OK.
- `git diff --check`: OK.
- `./install.sh`: OK.
- Servicio OpenRC `started`; `/api/health` responde `ok`.
- Hash instalado = hash `dist/onvif-visor`: `cec5b78215bf05b7db4d4d49240181c5f6ea52117f1c3de33ddd3ad3783826fc`.

# Limitación consciente
La detección sigue siendo a 1 FPS sobre el substream. Cuando el MKV principal todavía no ha escrito el cluster exacto, el fallback toma un snapshot inmediato del stream principal; por ello puede existir una diferencia temporal pequeña respecto al frame de detección, pero nunca vuelve a enviar la geometría del substream analítico.

# Resultado
Las alertas fotográficas de persona usan la geometría del stream principal que también ve el usuario en el panel. En las cámaras actuales, Telegram recibe 16:9 (`640x360`) en lugar del 4:3 (`640x480`) del substream NanoDet.
