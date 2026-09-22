# Contexto 16 — Grabación continua, rutas reales y detecciones en playback

## Objetivo

Se amplió la biblioteca/reproductor para soportar grabación continua 24/7 sin crear un segundo pipeline de FFmpeg, paginación con URL real y navegación directa a detecciones de personas dentro de cada archivo.

## Grabación continua

La grabación continua reutiliza el programador existente. No se agregó otro grabador ni otro stream RTSP por cámara.

Configuración canónica del modo continuo:
- schedule_enabled = true
- schedule_days = [0,1,2,3,4,5,6]
- schedule_start = "00:00"
- schedule_end = "00:00"
- schedule_timezone = zona horaria del navegador

La UI del diálogo de grabación ofrece tres modos:
- Sin grabación programada
- Continuo 24/7
- Por horario

El modo continuo corta el archivo al cambiar el día local. El scheduler revisa cada 2 segundos y, si detecta que el archivo activo pertenece al día local anterior, lo cierra y crea el nuevo archivo diario. Al salir del modo continuo se detiene el archivo continuo activo y se aplica inmediatamente la nueva programación.

La grabación por eventos sigue disponible y puede coexistir con el modo continuo; al existir un archivo continuo activo, las detecciones/eventos se asocian a ese mismo archivo.

## Protección de recursos

Los archivos de más de 6 horas no se transcodifican al cerrarse, evitando una recodificación completa de un archivo diario.

La analítica en vivo guarda progreso del archivo activo cada 10 segundos. Cuando un archivo de más de 6 horas termina y la cobertura en vivo llega a los últimos 15 segundos, se marca el análisis como completo conservando los segmentos detectados en vivo, sin volver a analizar las 24 horas. Si la analítica en vivo quedó incompleta, se conserva el fallback de análisis offline.

## Paginación con ruta real

La biblioteca admite:
- /grabaciones/{cameraID}/pagina/{page}
- query opcional ?fecha=YYYY-MM-DD

Los botones Anterior, Siguiente y los números de página son enlaces HTML reales a esas rutas. La ruta puede recargarse directamente y funciona con historial/back-forward. Si el backend ajusta una página fuera de rango, la URL se normaliza con replaceState a la página real.

El servidor registra explícitamente GET /grabaciones/{cameraID}/pagina/{page} como ruta de la aplicación.

## Playback y detecciones

El reproductor dedicado mantiene:
- /grabaciones/{cameraID}/{recordingName}
- query opcional ?at=segundos

Se corrigió el contenedor de video para centrar explícitamente la imagen con grid/place-items y object-position:center, manteniendo object-fit:contain para no recortar cámaras con otra relación de aspecto.

La API de objetos acepta recording_name opcional para consultar únicamente las detecciones del archivo abierto. Esto evita enviar todo el historial de detecciones al reproductor.

En playback:
- se cargan los segmentos de clase person del archivo abierto;
- se dibuja un punto amarillo por segmento en peak_seconds;
- cada punto es clicable y reinicia playback desde ese segundo;
- la URL se actualiza con ?at=;
- se muestra una leyenda con el número de detecciones de persona.

En las tarjetas de grabación:
- "Abrir archivo" abre desde 0;
- "Ir a persona" aparece cuando existe detección de persona y salta al mejor peak_seconds.

En un día con grabación continua se muestra "Abrir archivo del día", que abre el archivo más largo de la fecha seleccionada. La línea de tiempo inteligente existente sigue permitiendo saltar a cada segmento individual.

## Validación

Validaciones ejecutadas:
- gofmt sobre los Go modificados
- go test ./internal/server ./internal/media ./internal/analytics
- go test ./...
- go vet ./...
- esbuild de web/static/app.js
- git diff --check
- build CGO final
- smoke HTTP sobre producción

Smoke en 127.0.0.1:8081:
- /api/health -> 200
- /api/session -> 200
- /grabaciones/cam-prueba/pagina/3?fecha=2026-09-22 -> 200
- /grabaciones/cam-prueba/20260922-120000.000.mkv -> 200
- /app.js -> 200
- /api/recordings/summary sin sesión -> 401 esperado
- CSS desplegado contiene el centrado y estilos de marcadores
- servicio OpenRC -> started

No hay Chromium/Chrome headless instalado en el host, por lo que la comprobación visual final interactiva queda para un navegador real. La estructura, sintaxis, rutas, assets y smoke HTTP sí fueron verificados.

## Despliegue

Binario instalado:
- /opt/onvif-visor/onvif-visor
- SHA256: bff6d924ac241fa223e691370a78b29ab17e8a09e570a8687b24bc92858a996d

Backup previo:
- /opt/onvif-visor/backup-20260922-task30/onvif-visor
- SHA256: 537b6c07943da1bb6f09aa5c818e1212f976d5db03838817161d75b49818eb2a

Se conservó install.json y la escucha 0.0.0.0:8081.

README no se modificó; continúa siendo únicamente el resumen general del programa.
