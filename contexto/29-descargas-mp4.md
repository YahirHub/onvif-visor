# Fecha
2026-11-09

# Objetivo
Descargas de grabaciones en MP4, pedido prioritario del usuario: si la librería interna no podía, se debía usar FFmpeg.

# Decisiones tomadas
- Remux con FFmpeg sin recodificar: las grabaciones reales de las cámaras son HEVC 2304x1296 con audio AAC, ambos copiables a MP4 sin pérdida y al instante. No hace falta transcodificar vídeo nunca en este hardware.
- El HEVC se etiqueta hvc1, que es lo que esperan QuickTime y los reproductores de Apple. Si el audio no cabe en MP4 (G.711), solo se transcodifica el audio a AAC.
- La descarga clásica en Matroska se conserva sin el parámetro; el botón Descargar de la interfaz pide MP4 con ?format=mp4.
- MP4 fragmentado servido en streaming: no se genera ningún archivo intermedio y la descarga empieza sin esperar.

# Arquitectura actual
- Nuevo internal/media/mp4.go con mp4RemuxArgs, probeStreamCodecs con ffprobe y DownloadMP4 reutilizando el límite de reproducciones simultáneas. El handler de descarga deriva a MP4 solo si llega format=mp4.

# Librerías usadas
- Ninguna nueva. FFmpeg y ffprobe, que el proyecto ya exigía.

# Archivos importantes modificados
- internal/media/mp4.go y mp4_test.go (nuevos), internal/server/handlers.go, web/static/app.js.

# Problemas encontrados
- Ninguno en el remux: verificado con una grabación real del host (HEVC y AAC, duración conservada y MP4 válido reconocido por ffprobe).

# Soluciones implementadas
- Commit 1d5776f con tests de argumentos y de remux real generado en la suite. Desplegado en el host y verificado: tests de media y server en verde, servicio reiniciado y health correcto.

# Pendientes
- Los clips de Telegram ya viajan en su propio formato; no se han tocado.
- Confirmar con el usuario que el MP4 descargado se reproduce en su reproductor habitual.

# Próximos pasos
1. Confirmación del usuario de las descargas MP4.
2. Seguir con la fase 4 cuando se priorice.
