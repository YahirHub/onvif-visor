# Contexto 23 — CDN global de iconos y video de persona en Telegram

# Fecha
2026-09-22

# Objetivo
- Adoptar el nuevo CDN de iconos de forma global en la interfaz.
- Evitar mantener SVG manuales repetidos para iconos de UI.
- Permitir personalizar las alertas de persona de Telegram con foto, video o ambos.
- Añadir a las fotos de persona un botón inline "Ver video" asociado a la grabación/detección correcta.
- Generar el video desde el tramo real donde NanoDet mantuvo visible a la persona.

# Decisiones tomadas

## Iconos
Se adoptó Lucide 1.47.0 desde CDN, fijando versión:
- https://unpkg.com/lucide@1.47.0

Se usa data-lucide en el HTML y un helper iconMarkup() para contenido creado dinámicamente desde app.js.

refreshIcons() ejecuta lucide.createIcons() de forma agrupada con requestAnimationFrame. Esto evita llamadas repetidas en el mismo frame y permite convertir iconos agregados después de un render.

Los caracteres dentro de los elementos data-lucide se conservan como fallback visual si el CDN no carga.

Los SVG manuales usados antes como iconos se eliminaron. El único <svg> que permanece escrito en index.html es analyticsOverlay, porque es un lienzo dinámico para zonas y líneas, no un icono.

## Telegram: modos configurables
media_mode ahora admite:
- photo: foto de persona + botón "Ver video".
- video: video automático; para persona intenta usar el segmento real detectado.
- photo_video: foto con botón + video automático.

El límite configurable continúa siendo de 3 a 60 segundos. Para una persona, ese valor funciona como máximo: si la persona aparece menos tiempo, el clip se recorta a ese tramo.

Se conserva compatibilidad anterior:
- /foto y /video siguen disponibles.
- /video Cámara [segundos] mantiene su comportamiento.
- si un evento ONVIF no consigue confirmar una persona y el modo configurado es video, sigue enviando el fragmento genérico del evento como antes.

# Arquitectura actual

## Grabación necesaria para Telegram
Cuando Telegram confirma una alerta de persona:
1. se reutiliza la grabación activa de la cámara si ya existe;
2. de lo contrario se inicia una grabación con el mismo recorder principal;
3. Trigger extiende stopAfter hasta video_seconds + 8 s para dejar margen al detector y al merge de segmentos;
4. no se crea un segundo recorder paralelo por cámara.

Esto hace que el botón "Ver video" no dependa de tener activada previamente la opción de grabación por persona.

## Segmento exacto de persona
El video solicitado por Telegram se calcula desde object_segments:
- class_name = person;
- se busca el segmento más cercano al offset de la alerta;
- start_seconds define el inicio;
- end_seconds define el final;
- se espera a que el segmento lleve al menos 3.5 s sin crecer, coherente con el merge gap de 3 s;
- si el segmento alcanza el máximo configurado, se corta en ese máximo;
- si no puede obtenerse el segmento a tiempo, se usa un fallback alrededor del offset.

RecordingClip() convierte únicamente ese rango del MKV a MP4 H.264, 12 fps, ancho máximo 1280, y mantiene la geometría original sin crop.

## Botón "Ver video" e ID
Cada foto de persona crea una acción corta persistente:
- callback_data: pv:<id>
- id: 16 caracteres hexadecimales aleatorios
- duración: 7 días
- ligada al chat_id, camera_id, recording_name y offset_seconds

La tabla SQLite nueva es:
- telegram_media_actions

La acción está ligada mediante FK al chat de Telegram y no puede recuperarse desde otro chat.

Al pulsar el botón:
1. Telegram entrega callback_query.
2. El bot comprueba que el chat siga vinculado.
3. Busca la acción por id + chat.
4. Responde enseguida "Preparando video…".
5. Genera el clip de forma asíncrona.
6. Envía el MP4 al mismo chat.

El botón puede volver a usarse mientras la acción no expire y la grabación siga disponible. Si la retención ya eliminó el archivo, Telegram informa que el video ya no está disponible.

# Librerías usadas
- Lucide 1.47.0 vía CDN, sin dependencia npm local.
- Telegram Bot API existente.
- SQLite existente.
- FFmpeg existente.
- NanoDet/ONNX existente.

No se agregó paquete Go ni dependencia frontend al repositorio.

# Archivos importantes modificados
- internal/account/store.go
- internal/account/telegram.go
- internal/account/telegram_test.go
- internal/media/clip.go
- internal/media/clip_test.go
- internal/server/analytics.go
- internal/server/telegram_integration.go
- internal/server/telegram_integration_test.go
- internal/telegram/client.go
- internal/telegram/client_test.go
- web/static/index.html
- web/static/app.js
- web/static/styles.css

# Problemas encontrados
- Aún existían iconos tipográficos y SVG manuales fuera del sidebar.
- Telegram sólo aceptaba media_mode photo o video.
- GetUpdates sólo solicitaba updates de tipo message, por lo que no podía recibir callbacks de botones inline.
- Una foto de persona podía no tener una grabación suficiente para reconstruir después el tramo detectado.
- Durante el primer reemplazo del binario OpenRC reportó stop correcto, pero cp recibió text file busy durante la salida final del proceso.

# Soluciones implementadas
- Migración global de iconos de UI a data-lucide/iconMarkup.
- Carga fijada a Lucide 1.47.0.
- Fallback visual si falla el CDN.
- Nuevo media_mode photo_video.
- Soporte callback_query en el cliente Telegram.
- Envío multipart con reply_markup.
- AnswerCallbackQuery para feedback inmediato.
- Acciones persistentes y ligadas por chat.
- Grabación/extension automática para alertas de persona.
- Recorte desde object_segments.
- Reintento de despliegue una vez liberado el ejecutable tras el stop.

# Pruebas agregadas
- Persistencia de media_mode=photo_video.
- Persistencia y aislamiento por chat de TelegramMediaAction.
- Eliminación de acciones al borrar la integración.
- SendPhotoWithCallbackButton incluye inline_keyboard.
- GetUpdates solicita y decodifica callback_query.
- Ventana de grabación de persona.
- Normalización de duración para clips derivados de grabación.

# Validación
- gofmt: OK
- go test ./...: OK
- go vet ./...: OK
- esbuild app.js: OK, ~160.8 kB
- git diff --check: OK
- HTML sin IDs duplicados: OK
- CDN Lucide 1.47.0 accesible desde el servidor: HTTP 200
- index.html fuente contiene un solo SVG manual: analyticsOverlay, no un icono
- build CGO: OK

Smoke producción:
- /api/health -> 200
- /integraciones -> 200
- /app.js -> 200
- /styles.css -> 200
- /api/integrations/telegram sin sesión -> 401 esperado
- HTML desplegado contiene Lucide 1.47.0
- HTML desplegado contiene photo_video y "Ver video"
- JS desplegado contiene inicializador global de Lucide
- CSS desplegado contiene estilos .lucide
- servicio OpenRC: started

No hay Chrome/Chromium instalado en el host, por lo que la validación visual real de navegador queda limitada a checks HTML/JS/CSS y smoke HTTP.

# Despliegue
Binario instalado:
- /opt/onvif-visor/onvif-visor
- SHA256: 7827addb982cd42e8b3085d4ed294234741f282a3bae9dc18228ad0ecafdcec6

Backup:
- /opt/onvif-visor/backup-20260922-task37/onvif-visor
- SHA256: c99f6f3199454ec2161310a988ee0727e0e25b785b27138339d1ac44614a17c2

README no se modificó: este cambio se documenta en contexto/tareas y no convierte README en changelog.

# Pendientes
- Probar visualmente los iconos en un navegador real cuando exista Chrome/Chromium disponible.
- Probar con una detección real en Telegram para confirmar tiempos específicos de la cámara/RTSP y tamaño del MP4.
- Si la retención elimina la grabación antes de 7 días, el callback deja de poder reconstruir ese video; esto es intencional y se informa al usuario.

# Próximos pasos
- Verificar una alerta real de persona en cada modo: photo, video y photo_video.
- Confirmar que múltiples mensajes con botones "Ver video" recuperan cada uno su propio ID/clip.
