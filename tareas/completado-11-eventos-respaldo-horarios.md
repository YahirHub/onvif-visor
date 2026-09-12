# Tarea 11 — Eventos confiables y grabación programada

## Problema detectado
Las cámaras LC IPC-DK7-3H1WE anuncian soporte ONVIF PullPoint, pero devuelven un SOAP Fault `Subscribe Creation Failed` al crear la suscripción. La implementación anterior reintentaba internamente y no propagaba el fallo, por lo que la UI podía quedar sin eventos sin mostrar una causa clara.

## Solución
- Propagar errores reales de la suscripción ONVIF PullPoint.
- Mantener un watcher por cámara aunque la grabación por evento esté desactivada, para que la vista Eventos siga recibiendo actividad.
- Cuando PullPoint falla, usar un detector local de movimiento basado en FFmpeg sobre un substream de baja carga (2 fps, 160x90, escala de grises) y compensación de cambios globales de luminosidad.
- Emitir eventos de inicio/fin de movimiento y usar los eventos activos para disparar grabaciones si `event_recording` está habilitado.
- Evitar que un nuevo disparo acorte una grabación ya activa.
- Iniciar watcher inmediatamente al agregar una cámara.

## Programación
- Añadir configuración persistente por cámara con días de la semana, hora de inicio, hora final y zona horaria IANA.
- Soportar horarios que cruzan medianoche.
- Arrancar/estirar la grabación hasta el final de la ventana programada sin interferir con el post-roll de eventos.
- Añadir endpoint `PUT /api/cameras/{id}/schedule`.
- Añadir diálogo en la galería para activar/desactivar horarios, seleccionar días y horas.

## Verificación
- Se reprodujo el fallo real de PullPoint en ambas cámaras LC.
- El detector de respaldo generó un evento real `SoftwareMotion/Motion` con score 0.0611.
- Ese evento inició una grabación MKV real en la cámara LC con la grabación por movimiento habilitada.
- `go test ./...`, `go vet ./...`, build y `git diff --check` pasan.
