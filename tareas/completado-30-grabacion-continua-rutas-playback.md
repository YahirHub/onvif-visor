# Tarea 30 — Grabación continua, rutas reales y playback con detecciones

## Estado
Completada y desplegada.

## Implementado
- Reproductor dedicado centrado con object-fit: contain y object-position:center.
- Paginación de Grabaciones con ruta real /grabaciones/{cameraID}/pagina/{page}?fecha=YYYY-MM-DD.
- Anterior, Siguiente y números de página son enlaces HTML reales, no cambios de página por listeners JavaScript.
- Modo de grabación Continuo 24/7 en el diálogo de configuración, reutilizando el scheduler y el único grabador existente.
- Corte diario por zona horaria local y aplicación inmediata al cambiar configuración.
- Protección de CPU: archivos continuos largos no se recomprimen y la analítica en vivo puede cerrar el análisis sin reprocesar 24 horas.
- Filtro recording_name en /api/analytics/objects.
- Puntos amarillos clicables de detección de persona en la barra de reproducción.
- Leyenda con número de detecciones.
- Acciones "Abrir archivo" e "Ir a persona" en cada grabación.
- Acción "Abrir archivo del día" para grabaciones continuas/largas.
- Ruta SPA del servidor para páginas reales.

## Validación
- gofmt: OK.
- go test ./internal/server ./internal/media ./internal/analytics: OK.
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Build CGO: OK.
- Smoke de producción: OK.
- Servicio OpenRC: started.
- Sin navegador headless disponible en el host; comprobación visual interactiva pendiente de navegador real.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: bff6d924ac241fa223e691370a78b29ab17e8a09e570a8687b24bc92858a996d
- Backup: /opt/onvif-visor/backup-20260922-task30/onvif-visor
- SHA256 backup: 537b6c07943da1bb6f09aa5c818e1212f976d5db03838817161d75b49818eb2a
- README sin cambios.
