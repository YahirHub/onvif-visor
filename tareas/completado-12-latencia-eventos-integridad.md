# Tarea 12 — Baja latencia, eventos navegables e integridad de grabaciones

## Objetivo
Corregir el atraso acumulativo del live, hacer multivista y eventos incrementales sin parpadeos, enlazar eventos al instante exacto de su clip y endurecer el almacenamiento ante cortes de energía.

## Alcance
- Live MJPEG con descarte de cuadros antiguos para impedir que un cliente lento acumule minutos de atraso.
- Parámetros RTSP/FFmpeg de baja latencia y perfil reducido para nuevas cámaras en multivista.
- Multivista incremental: agregar/quitar una cámara no reinicia streams que ya están activos.
- Sondeo ligero de cámaras para que altas/bajas aparezcan en el selector sin recargar toda la vista.
- Eventos con miniatura y referencia al clip/segundo de la grabación.
- Click en evento -> abrir grabaciones en ese punto temporal.
- Polling de eventos más frecuente e incremental, con animación sólo para entradas nuevas.
- MKV con clusters/flush frecuentes para mejorar recuperación ante interrupciones.
- Compresión post-grabación en segundo plano y reemplazo atómico sólo si el archivo resultante es menor y válido.
- Mantener el live y la grabación desacoplados para que la compresión no provoque pérdida de cuadros.

## Criterios de aceptación
- Un cliente lento no hace que el live continúe reproduciendo cuadros viejos indefinidamente.
- Agregar una cámara a multivista conserva intactos los streams existentes.
- Eventos grabados exponen miniatura y offset reproducible.
- La compresión nunca sustituye el original hasta terminar exitosamente.
- `go test ./...`, `go vet ./...`, build y `git diff --check` pasan.

## Resultado
Completado y validado. El live ya no entrega directamente la cola de FFmpeg al navegador: separa JPEGs, conserva como máximo un cuadro pendiente y descarta el anterior cuando el cliente no alcanza el ritmo, evitando atraso acumulativo. La vista individual usa 8 fps/960 px y las nuevas cámaras agregadas a multivista usan un perfil ligero de 5 fps/640 px; además, agregar o quitar una cámara ya no reinicia los streams existentes.

Los eventos ahora guardan el nombre del clip y el offset temporal cuando existe una grabación asociada. La UI consulta eventos cada 2.5 s de forma incremental, presenta miniaturas y permite abrir directamente la grabación en el segundo del evento. Las cámaras se resincronizan cada 4 s sin reconstruir la multivista si no cambió la selección.

Las grabaciones usan clusters Matroska de 1 s, flush de paquetes y `fsync` periódico cada 3 s. Al cerrarse se intenta una compresión H.264 CRF 24 en segundo plano, limitada a un trabajo y un thread; el original sólo se sustituye mediante rename atómico cuando el resultado termina correctamente y ocupa al menos 3% menos. En la prueba real, la cámara entregó HEVC ya eficiente y el sistema conservó el original porque una recompresión no aportaba ahorro, evitando degradación innecesaria.

Verificación real: el endpoint live ligero entregó HTTP 200 con multipart `onvifframe`, `X-Accel-Buffering: no` y aproximadamente 849 kB en 5 s. Un movimiento real generó `SoftwareMotion/Motion`, creó el MKV `20260912-204620.769.mkv`, el evento posterior quedó ligado a ese clip con offset 1.99 s y la miniatura exacta respondió HTTP 200 con 49,290 bytes. `go test ./...`, `go vet ./...`, build estático y `git diff --check` pasan. La comprobación visual final en navegador sigue pendiente porque este host no dispone de navegador/runtime JavaScript.
