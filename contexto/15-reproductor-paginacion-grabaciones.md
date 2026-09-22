# Contexto 15 — Reproductor dedicado y paginación de grabaciones

## Decisión principal
La biblioteca de **Grabaciones** ya no descarga el histórico completo de todas las cámaras ni mantiene el reproductor dentro de la misma vista.

Se separaron dos responsabilidades:
- `/grabaciones/<camera-id>` conserva biblioteca, fecha, timeline diario, analítica y retención.
- `/grabaciones/<camera-id>/<archivo.mkv>` abre una pantalla dedicada de reproducción.

Los saltos desde timeline, detecciones y Eventos usan la ruta dedicada y pueden incluir `?at=<segundos>` para iniciar en el instante correspondiente.

## API de grabaciones
`GET /api/recordings` ahora requiere `camera_id` y devuelve un objeto paginado:
- `items`: clips de la página actual.
- `timeline`: clips que intersectan el rango solicitado para mantener el día completo visible.
- `page`, `page_size`, `total`, `total_pages`.

Parámetros:
- `page`: mínimo 1.
- `page_size`: 1–100; frontend usa 24.
- `from` y `to`: RFC3339; si se usan, ambos son obligatorios.
- El rango de timeline se limita a 48 h.

`GET /api/recordings/summary` entrega acumulados por cámara y almacenamiento total sin enviar todos los clips al navegador.

## Frontend
- La biblioteca consulta una cámara y una fecha cada vez.
- Los clips visibles se paginan de 24 en 24.
- La paginación muestra anterior/siguiente y páginas cercanas con elipsis.
- El timeline conserva todos los clips del día seleccionado.
- El reproductor dedicado mantiene pausa/reanudación, seek, descarga y eliminación.
- El botón volver regresa a la biblioteca de la misma cámara.
- Eventos y segmentos de analítica abren directamente la nueva ruta de reproducción.

## Compatibilidad y seguridad
Se mantiene el modelo existente `camera_id + basename .mkv`; no se exponen rutas físicas del host ni se relaja autenticación.
Las rutas nuevas de SPA siguen detrás del mismo shell y los endpoints de reproducción/metadata/descarga permanecen autenticados.

## Validación
- `gofmt`: OK.
- `go test ./...`: OK.
- `go vet ./...`: OK.
- `git diff --check`: OK.
- build CGO: OK.
- build `CGO_ENABLED=0`: OK.
- esbuild sobre `web/static/app.js`: OK.
- auditoría DOM: 223 IDs HTML, 211 referencias directas JS, `missing=[]`.
- smoke temporal: ruta dedicada y sesión HTTP 200.
- pruebas nuevas cubren query de paginación, rango/solapamiento, clamp de página, resumen y ruta SPA dedicada.

## Despliegue 2026-09-22
La instalación existente usaba OpenRC y `0.0.0.0:8081`.

Se compiló el binario final y se reemplazó el servicio existente sin tocar `data/`, `recordings/` ni `install.json`.
- SHA-256 nuevo: `537b6c07943da1bb6f09aa5c818e1212f976d5db03838817161d75b49818eb2a`.
- Binario anterior respaldado en `/opt/onvif-visor/backup-20260922-task29/onvif-visor`.
- SHA-256 anterior: `7dfe48c56137f4a69981483087ea3c85b62983c94babdaf8f6e49bff64ffdfc6`.
- OpenRC volvió a estado `started`.
- Producción: `/api/health` 200, `/api/session` 200, ruta de playback 200, `/app.js` 200.
- `/api/recordings/summary` sin sesión devuelve 401, conservando protección.

La validación visual final continúa dependiendo de un navegador real; el host no dispone de navegador automatizado.
