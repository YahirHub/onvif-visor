# Tarea 10 — UI VMS minimalista y rutas reales

## Objetivo
Reducir al mínimo el chrome visual para que cámaras, multivista y reproducción sean los protagonistas, y convertir la navegación interna en rutas reales recargables.

## Cambios solicitados
- Eliminar exceso de bordes, tarjetas y contenedores visibles.
- Reducir encabezados y sidebar.
- Convertir las miniaturas en superficies de vídeo con metadatos superpuestos.
- Hacer que la multivista ocupe prácticamente todo el workspace; selector de cámaras como drawer superpuesto.
- Simplificar grabaciones/timeline para que vídeo y timeline destaquen por encima de paneles decorativos.
- Rutas reales: `/camaras`, `/camara/{id}`, `/multivista`, `/grabaciones`, `/grabaciones/{cameraID}` y `/eventos`.
- Soportar recarga directa y navegación back/forward.

## Criterios de aceptación
- Las rutas anteriores sirven `index.html` directamente desde Go y funcionan al recargar.
- El frontend sincroniza URL ↔ vista con History API.
- El monitor individual usa `/camara/{id}`.
- La cámara seleccionada en almacenamiento usa `/grabaciones/{cameraID}`.
- La galería ya no muestra endpoint ni bloques separados debajo del vídeo.
- En multivista el selector no resta ancho permanentemente al vídeo.
- `gofmt`, `go test ./...`, `go vet ./...` y build estático pasan.

## Resultado
Completado. La interfaz quedó reducida a vídeo/miniaturas como foco principal, multivista con selector superpuesto y rutas recargables servidas por Go.
