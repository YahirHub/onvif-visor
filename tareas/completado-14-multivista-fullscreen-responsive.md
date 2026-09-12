# Tarea 14 — Multivista estable, fullscreen y responsividad

## Objetivo
Corregir el cambio de relación de aspecto al agregar cámaras dinámicamente, añadir pantalla completa sin sidebar y endurecer la responsividad móvil.

## Alcance
- Mantener la relación original del stream al entrar/salir de multivista.
- Evitar usar un substream con relación distinta al stream principal.
- Recalcular layout con ResizeObserver y sin parpadeos.
- Botón Pantalla completa con Fullscreen API y fallback modo teatro.
- Responsive para móvil/tablet con navegación compacta y controles táctiles.

## Resultado
Completado. El perfil ligero de multivista ya no cambia al substream del firmware: conserva el mismo RTSP y reduce FPS/resolución después de decodificar, eliminando la causa principal del cambio 1:1. Los panes eliminados salen inmediatamente del grid para no crear celdas implícitas, y los nuevos conservan animación de entrada. `ResizeObserver` recalcula el stage al cambiar el contenedor. Se agregó fullscreen real sobre `monitorView`, con fallback modo teatro que oculta el sidebar. En móvil el monitor usa panes 16:9 con `object-fit: contain`, navegación inferior desplazable para seis módulos, safe-area y layouts específicos hasta 420 px.
