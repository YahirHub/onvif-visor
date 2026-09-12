# Tarea 03 — Visualización en vivo

## Objetivo
Mostrar vídeo RTSP de cámaras agregadas en el navegador con baja latencia mediante FFmpeg y MJPEG.

## Criterios de aceptación
- Endpoint live por cámara con cancelación al cerrar cliente.
- FFmpeg sin credenciales visibles en logs.
- Límite de procesos/conexiones por cámara.
- Mensajes claros si FFmpeg no está instalado o el stream falla.
- UI responsive para varias cámaras.

## Resultado
Completado y verificado: live RTSP->MJPEG con FFmpeg, cancelación al cerrar cliente, máximo 4 espectadores por cámara y UI responsive embebida.
