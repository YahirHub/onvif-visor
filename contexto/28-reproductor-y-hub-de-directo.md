# Fecha
2026-11-09

# Objetivo
Tres mejoras pedidas por el usuario en orden: commitear el trabajo, arreglar el reproductor que colapsaba al buscar, y optimizar el directo y la multivista con un hub compartido.

# Decisiones tomadas
- Control de versiones en el clon del sandbox con commits en español y autor ThowiLabs: 78628f6 diseño, 8097a02 reproductor, 8d5ebbe hub, a454ef7 arreglo del hub y 1d5776f descargas MP4.
- Reproductor: el marco conserva siempre su aspecto 16/9 y al buscar, pausar o terminar se congela el último fotograma en un póster tras el stream. El indicador de carga es una pastilla pequeña, nunca un panel que sustituya al vídeo.
- Hub de directo: un solo FFmpeg por cámara y perfil (full o low) alimenta a todos los espectadores. Cada suscriptor guarda solo el fotograma más reciente: un cliente lento salta fotogramas viejos en vez de acumular retraso. Si la cámara se corta, el hub relanza el proceso con espera creciente sin cerrar la conexión de nadie. Gracia de 3 s sin espectadores antes de apagar el FFmpeg.

# Arquitectura actual
- internal/media: liveHub con publish/subscribe en live.go, mapa de hubs en el Manager y argumentos en liveFFmpegArgs. El resto igual.

# Librerías usadas
- Ninguna nueva.

# Archivos importantes modificados
- web/static/app.js, index.html, styles.css: póster del reproductor y marco con aspecto fijo.
- internal/media/live.go, live_test.go, manager.go: el hub y sus tests.

# Problemas encontrados
- El hub desplegado dejó el directo sin fotogramas en producción: se había añadido -rw_timeout a FFmpeg, opción que no existe en el FFmpeg 9 del host, y el proceso moría al abrir la entrada. En el sandbox, con FFmpeg 7, la opción sí existe y las pruebas pasaban.
- La primera prueba extremo a extremo en el sandbox no pudo usar una cámara real y un archivo local no sirve como entrada RTSP, así que el fallo solo apareció en el host.

# Soluciones implementadas
- Commit a454ef7: fuera -rw_timeout y cuelgues detectados con un vigilante de silencio de 15 s en el propio hub que mata y relanza el proceso. Test de regresión sobre los argumentos en live_test.go.
- Verificación definitiva en el host antes de redesplegar: prueba desechable con la cámara real, dos espectadores simultáneos recibiendo MJPEG a través de exactamente un hub, en verde.
- Lección registrada: cualquier opción de FFmpeg se prueba antes contra el binario del host, no contra el del sandbox.

# Pendientes
- deuda-tecnica: la analítica sigue abriendo su propio stream RTSP por cámara; unirla al hub es otro paso, con cuidado porque muestrea distinto (grises y bgr24 a 1-2 fps).
- El usuario debe confirmar en su navegador que el directo y la multivista se ven fluidos.

# Próximos pasos
1. Confirmación visual del usuario del directo y la multivista.
2. Fase 4 cuando se priorice: app.js por módulos y navegación de cuatro secciones.
