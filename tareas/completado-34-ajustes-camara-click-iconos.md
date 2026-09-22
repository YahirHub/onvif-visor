# Tarea 34 — Abrir Ajustes de cámara y centrar acciones

## Estado
Completada y desplegada.

## Implementado
- openCameraSettings(cameraID) asigna la cámara antes de navegar.
- Ruta real /camaras/{id}/configuracion conservada.
- Ajustes usa preventDefault y stopPropagation.
- Eliminar también detiene propagación.
- camera-hover-actions con z-index propio.
- Ajustes y Eliminar reemplazados por SVG inline.
- Botones 30x30 con centrado independiente de la fuente.

## Validación
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Build CGO: OK.
- Smoke de producción: OK.
- Servicio OpenRC: started.
- README sin cambios.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: 43dac60dea4b7af61749f2192f25940b7b6ce9f4aca5ae8f6ac709d08949a7b8
- Backup: /opt/onvif-visor/backup-20260922-task34/onvif-visor
- SHA256 backup: d974e6d5c66ddc9a2c1c862acf1ddb3e13c785730873e192c4f2626bea455044
