# Tarea 08 — Grabaciones, reproducción y línea de tiempo

## Objetivo
Crear un apartado de almacenamiento por cámara para consultar, reproducir y ubicar grabaciones en una línea de tiempo profesional.

## Alcance
- Indexar archivos MKV ya generados por el grabador.
- Resumen por cámara: cantidad, tamaño y duración aproximada.
- Selector de cámara y fecha.
- Línea de tiempo de 24 horas con bloques de grabación.
- Reproductor web de grabaciones mediante FFmpeg -> MJPEG con seek por segundos.
- Miniatura JPEG por grabación.
- Descarga segura del MKV original.
- Límites de concurrencia de reproducción y validación estricta de nombres/rutas.

## Criterios de aceptación
- No exponer rutas físicas del host en la API.
- No permitir path traversal.
- Click en una cámara muestra únicamente sus grabaciones.
- Click en un bloque o tarjeta abre la reproducción.
- La línea de tiempo representa la hora/duración del clip.
- El scrubber reinicia la reproducción desde el punto elegido.
- `gofmt`, `go test ./...`, `go vet ./...` y build estático pasan.

## Resultado
Completado y validado. Se agregó biblioteca por cámara, resumen de clips/espacio/duración, filtros por fecha, línea de tiempo de 24 horas, miniaturas de clips, reproducción MJPEG con seek, pausa/reanudación y descarga del MKV original. Las rutas físicas del host no se exponen y los nombres se validan para bloquear path traversal. La reproducción está limitada a 4 sesiones simultáneas. Las pruebas unitarias cubren traversal y parsing/listado de grabaciones; el endpoint de biblioteca respondió correctamente en smoke test local.

No se dispone todavía de un clip real generado por una cámara física en este entorno para validar la reproducción extremo a extremo.
