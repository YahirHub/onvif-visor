# Tarea 19 — Vídeo Telegram optimizado y duración configurable

## Objetivo
Corregir clips Telegram cortos/borrosos y permitir duración por defecto configurable y override por comando.

## Alcance
- Duración por defecto configurable de 3 a 60 s en Integraciones.
- `/video <camara>` usa duración configurada.
- `/video <camara> <segundos>` admite override, por ejemplo `/video Cocina 30`.
- Parser seguro compatible con nombres de cámara con espacios.
- Captura MP4 con timestamps reiniciados, warm-up breve, calidad 720p aprox., H.264 y carga acotada.
- Timeout proporcional a duración, no fijo.
- Mantener una sola captura Telegram simultánea.
- Pruebas del parser y validación real/sintética del clip.

## Resultado
Completado. La duración configurable pasa a 3–60 s y `/video <cámara> <segundos>` permite override manteniendo el valor del panel como default. El comando intenta primero resolver el nombre completo de cámara para no confundir nombres numéricos y luego interpreta el último argumento como duración. Los clips ya no usan el perfil agresivo `nobuffer/low_delay`: se generan con timestamps de reloj, `setpts`, warm-up de 500 ms, 12 fps, máximo 1280 px, H.264 CRF 23 con tope 1.8 Mbps, GOP corto y timeout proporcional. El long polling no se bloquea mientras se genera un vídeo y sólo se permite una captura Telegram simultánea. Prueba real con Cocina: 6.000 s exactos, H.264 1280x720 12 fps; prueba real de `/video ... 30`: 30.000 s exactos y 6,901,003 bytes. Las pruebas unitarias de límites, parser y configuración pasan.
