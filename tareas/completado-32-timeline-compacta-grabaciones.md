# Tarea 32 — Timeline compacta en Grabaciones

## Estado
Completada y desplegada.

## Implementado
- Timeline compacta por defecto con solo Grabación + Personas.
- Marcadores amarillos para Persona.
- Botón "Ver todas las clases" / "Mostrar solo personas".
- Se mantienen zoom, fecha, reanálisis y saltos a video.
- Resumen contextual de detecciones debajo de la timeline.
- Resumen diario visualmente más compacto.
- Archivos movidos inmediatamente debajo de la timeline.
- Controles de eliminación movidos después de archivos y paginación.
- Vista compacta responsive.

## Validación
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Smoke de producción: OK.
- Servicio OpenRC: started.
- README sin cambios.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: 1afb590d13a6ca5a62aa9c187b2d8b55096fcfd880112e5d50f95173f7ae8c74
- Backup: /opt/onvif-visor/backup-20260922-task32/onvif-visor
- SHA256 backup: 67e3d3e2f1c12a93b73d2ccc339e8c045d4cbee7b7dc3adcf53018b5c7ec6c0b
