# Tarea 29 — Reproductor dedicado y paginación de grabaciones

## Estado
Completada y desplegada.

## Implementado
- Reproductor retirado de la vista principal de Grabaciones.
- Nueva ruta dedicada `/grabaciones/<camera-id>/<archivo.mkv>`.
- Soporte de offset directo `?at=` desde Eventos y analítica.
- API de listado paginada por cámara, fecha y página.
- Resumen separado por cámara para contadores/espacio/duración.
- Timeline diario conserva todos los clips del rango aunque las tarjetas estén paginadas.
- Paginación inteligente de 24 clips con anterior/siguiente, páginas cercanas y elipsis.
- Seek, pausa/reanudación, descarga y eliminación conservados en la pantalla dedicada.
- Borrado por lote recarga resumen/listado desde servidor para no depender de un histórico completo en memoria.
- Pruebas de paginación, rango, resumen y ruta SPA añadidas.
- README restaurado como resumen general del programa; el detalle técnico queda únicamente en `contexto/` y `tareas/`.

## Validación
- `go test ./...`: OK.
- `go vet ./...`: OK.
- `git diff --check`: OK.
- build CGO y estático: OK.
- esbuild frontend: OK.
- auditoría DOM: sin IDs directos faltantes.
- smoke HTTP temporal: OK.
- smoke producción: health/session/UI/assets 200 y API protegida 401 sin cookie.

## Despliegue
- Servicio existente OpenRC detenido.
- Binario anterior respaldado en `/opt/onvif-visor/backup-20260922-task29/`.
- Nuevo binario instalado en `/opt/onvif-visor/onvif-visor`.
- SHA-256 instalado: `537b6c07943da1bb6f09aa5c818e1212f976d5db03838817161d75b49818eb2a`.
- Servicio reiniciado correctamente en `0.0.0.0:8081`.
