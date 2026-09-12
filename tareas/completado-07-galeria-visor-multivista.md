# Tarea 07 — Galería, visor y multivista

## Objetivo
Rediseñar el dashboard post-login como un VMS profesional con miniaturas de cámaras y un visor dedicado individual/multivista.

## Alcance
- Shell de panel con sidebar y navegación clara.
- Galería de cámaras tipo thumbnails con snapshot real, nombre, fabricante/modelo y estado.
- Click en una miniatura abre el visor en vivo.
- Modo multivista con selección dinámica de cámaras.
- Layout inteligente: 1, 2, 3, 4, 5-6 y 7-9 cámaras.
- Selector avanzado para agregar/quitar cámaras durante la reproducción.
- Mantener login frontend, sesión HttpOnly y modales propios.
- Snapshot de cámara mediante FFmpeg con límites/timeouts.

## Criterios de aceptación
- No se abren streams live en la galería: solo snapshots para reducir consumo.
- Un click abre live individual.
- Dos cámaras ocupan mitad y mitad; 3 usan layout principal+2; 4 2x2; hasta 9 se adaptan.
- La sesión expirada vuelve al login visual.
- UI responsive y sin dependencias frontend nuevas.

## Resultado
Completado y validado: galería post-login con snapshots JPEG obtenidos bajo demanda mediante FFmpeg, visor individual, multivista seleccionable hasta 9 cámaras y layouts automáticos 1/2/3/4/5-6/7-9. Se conserva el login frontend y el consumo de live se limita al visor, no a la galería. `gofmt`, `go test ./...`, `go vet ./...` y build estático pasan.
