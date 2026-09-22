# Tarea 31 — Playback sin recorte

## Estado
Completada y desplegada.

## Implementado
- Eliminado aspect-ratio 16:9 forzado del reproductor.
- El contenedor ya no limita ni recorta el cuadro.
- La imagen conserva width/height automáticos y solo se limita con max-width/max-height.
- object-fit:contain y object-position:center center.
- Miniaturas de Grabaciones cambiadas de cover a contain.
- Verificación con archivo real 2304x1296 -> playback 1280x720 sin cambio de proporción.
- Pipeline FFmpeg sin cambios porque no realizaba crop.

## Validación
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Smoke de producción: /api/health 200, ruta de playback 200.
- CSS desplegado sin aspect-ratio forzado en el reproductor.
- Servicio OpenRC: started.
- README sin cambios.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: 67e3d3e2f1c12a93b73d2ccc339e8c045d4cbee7b7dc3adcf53018b5c7ec6c0b
- Backup previo: /opt/onvif-visor/backup-20260922-task31/onvif-visor
- SHA256 backup: bff6d924ac241fa223e691370a78b29ab17e8a09e570a8687b24bc92858a996d
