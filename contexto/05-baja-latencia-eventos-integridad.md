# Fecha
2026-09-12

# Objetivo
Eliminar el atraso acumulativo del live, hacer multivista/eventos incrementales, enlazar eventos con el segundo exacto de la grabación y endurecer la escritura/compresión de clips.

# Diagnóstico
- El live anterior conectaba stdout MJPEG de FFmpeg directamente a `io.Copy`. Si el navegador o la red consumían más lento que FFmpeg, la presión de salida podía conservar cuadros antiguos y el usuario terminaba viendo vídeo retrasado en vez del cuadro actual.
- `renderMonitor()` destruía y recreaba todos los `<img>` live al agregar/quitar una cámara, provocando parpadeo y reconexiones innecesarias.
- Eventos se consultaba cada 10 s y se reconstruía todo el listado; no tenía miniatura ni referencia a la grabación.
- Las grabaciones eran MKV `-c copy` sin clusters cortos/fsync periódico ni optimización de espacio post-proceso.

# Soluciones
- Live FFmpeg ahora entrega JPEG puro al servidor. El servidor separa cada JPEG y usa una cola de capacidad 1: si el cliente se atrasa, sustituye el frame pendiente por el más reciente. Así la latencia no crece de forma indefinida.
- Parámetros de baja latencia: RTSP TCP, `nobuffer`, `low_delay`, `analyzeduration=0`, `probesize=32768`, `max_delay=0`.
- Vista individual: 8 fps, máximo 960 px. Nuevas cámaras en multivista: substream cuando está disponible, 5 fps, máximo 640 px.
- Multivista incremental: panes existentes se conservan; sólo se crea o elimina la cámara modificada. Se agregaron animaciones breves de entrada/salida y loading overlay.
- Lista de cámaras se resincroniza cada 4 s y eventos cada 2.5 s.
- Cada evento puede guardar `recording_name` y `recording_offset`; la UI genera miniatura desde ese punto del MKV y al hacer click navega a la biblioteca y comienza reproducción desde ese offset.
- Las miniaturas de evento reintentan mientras el clip está recién creado y luego degradan a snapshot de cámara si todavía no es legible.
- Grabación MKV mantiene `-c copy` durante captura para no consumir CPU ni introducir pérdida. Se usan clusters de 1 s, `flush_packets=1`, cluster máximo 1 MiB y `fsync` del archivo cada 3 s.
- Al terminar un clip se ejecuta compresión en background, máximo una a la vez y un thread. Se intenta H.264 CRF 24/preset veryfast, pero sólo se reemplaza el original mediante rename atómico si el resultado ocupa al menos 3% menos. Si el origen ya es HEVC eficiente, se conserva el original para no perder calidad ni aumentar tamaño.

# Verificación real
- Live ligero respondió 200, multipart `onvifframe`, `X-Accel-Buffering: no`; recibió aproximadamente 849 kB en 5 segundos.
- El detector real de movimiento generó un clip `20260912-204620.769.mkv`.
- Un evento posterior quedó asociado a ese clip en offset 1.99 s.
- La miniatura del evento a 1.99 s respondió 200 y 49,290 bytes.
- `ffprobe` confirmó que ese clip de la cámara ya estaba en HEVC 2304x1296; la compresión no sustituyó el original porque no mejoraba el tamaño.
- `go test ./...`, `go vet ./...`, build estático y `git diff --check` pasan.

# Limitación residual
Ningún contenedor puede garantizar cero corrupción frente a cualquier corte de energía. Los clusters cortos, flush y fsync reducen la ventana de datos en riesgo, pero para máxima protección sigue siendo recomendable un UPS y un filesystem con journaling. La validación visual final de animaciones/DOM queda pendiente porque el host de desarrollo no dispone de navegador gráfico ni runtime JavaScript.
