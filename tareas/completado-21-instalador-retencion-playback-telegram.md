# Tarea 21 — Instalación automática, playback exacto, Telegram con persona y retención

## Objetivo
Unificar la instalación desde código fuente, corregir el final del reproductor, hacer que las alertas Telegram de foto usen un frame confirmado por NanoDet sin deformarlo y añadir eliminación segura de grabaciones individuales o por periodos, incluyendo limpieza consistente de eventos y miniaturas inteligentes.

## Implementado
- Nuevo `install.sh` ejecutable y POSIX `sh`.
- Detecta `apk`, `apt`, `dnf`, `yum`, `pacman` o `zypper`.
- Instala toolchain C/Go, Make, Git, curl, certificados, tar/gzip y FFmpeg/ffprobe cuando faltan.
- Si el Go disponible es anterior a 1.24 descarga la versión estable desde `go.dev` para amd64/arm64.
- ONNX Runtime se reutiliza si ya existe, intenta paquete nativo primero y, en Linux glibc amd64/arm64, descarga automáticamente el release oficial de Microsoft como fallback. En musl no instala binarios glibc incompatibles.
- Ejecuta `go mod download`, `go test ./...`, `go vet ./...`, `make build` y finalmente `onvif-visor --install`, pasando la ruta exacta de ONNX Runtime.
- Es idempotente: si el toolchain ya está completo no reinstala paquetes. Incluye reintentos para fallos transitorios del gestor.
- `ONVIF_SKIP_TESTS=1` permite omitir únicamente tests/vet de forma explícita.

## Playback exacto
- `media.Manager` autodetecta `ffprobe` junto a FFmpeg o en PATH.
- Un clip cerrado obtiene duración exacta mediante `ffprobe format=duration` al abrirse en el reproductor.
- La duración se cachea por path+tamaño+mtime; no se ejecuta ffprobe para cada refresco de toda la biblioteca.
- El frontend espera esa metadata antes de iniciar playback y usa el valor exacto para total, scrubber y final.
- Si el stream MJPEG termina, el timer se detiene inmediatamente; un EOF cercano a la duración real se trata como final normal y no como error visual.
- Si ffprobe no está disponible o falla, se conserva el fallback anterior.

## Telegram con NanoDet
- El stream live de analítica ya no genera la imagen notificable desde el canvas 320x320 del modelo.
- `ffprobe` obtiene ancho/alto del substream; FFmpeg entrega un frame BGR reducido manteniendo su relación de aspecto (por ejemplo 320x180 para 16:9).
- Go crea aparte un letterbox 320x320 únicamente para inferencia NanoDet. El JPEG que se envía a Telegram conserva las dimensiones/aspecto del frame antes del letterbox.
- En memoria sólo se conserva el último JPEG cuyo frame haya sido clasificado como `person`, junto con hora y confianza; no se persisten imágenes adicionales.
- Para alertas automáticas en modo foto se espera hasta 10 s un frame `person` relacionado con el evento (hasta 3 s previos) y se envía exactamente ese JPEG.
- Si no aparece ninguna persona, no se envía una captura genérica/falsa.
- `/foto <camara>` manual continúa como snapshot inmediato y `/video` mantiene su lógica independiente.
- Se corrigió el selector de **Medio de las alertas automáticas**: el polling de cámaras ya no vuelve a cargar la configuración cada 4 s mientras el usuario edita, por lo que `Fragmento de vídeo` puede seleccionarse y guardarse.
- La duración de vídeo queda siempre editable incluso si el medio automático es foto, porque también es el valor por defecto de `/video Cámara`.
- La UI aclara que el selector de medio sólo afecta alertas automáticas y no cambia `/foto` ni `/video`.

## Miniaturas de personas
- En Grabaciones, cada tarjeta busca el segmento NanoDet de mayor confianza de ese MKV y pide el snapshot exactamente en `peak_seconds`.
- Si el snapshot de persona no puede generarse, cae a la miniatura normal del clip.
- Las tarjetas con detección muestran badge `PERSONA <confianza>%`.
- `loadRecordingAnalytics` sólo vuelve a renderizar timeline/tarjetas cuando cambia la huella de segmentos, evitando recargas y flicker cada 4 s.
- `/api/events` enriquece eventos asociados a MKV con `person_offset` y `person_confidence`, usando el mejor segmento del clip.
- Las miniaturas de Eventos usan ese `person_offset`; si la persona aparece después de crear la tarjeta, el render incremental detecta el cambio de huella y reemplaza sólo esa miniatura.

## Retención, borrado y eventos
- Nuevo `DELETE /api/recordings/{cameraID}/{name}` para un clip específico.
- Nuevo `POST /api/recordings/delete-batch` para borrado por rango o todas las grabaciones cerradas de una cámara.
- Nuevo `GET /api/recordings/{cameraID}/{name}/metadata` para duración exacta.
- La API nunca recibe paths físicos; usa camera ID + basename `.mkv` validado.
- Un archivo activo devuelve conflicto/no se borra; en lote se omite y se informa `skipped_active`.
- Tras borrar se sincroniza el directorio y se limpia la caché de duración.
- Se eliminan `recording_analysis` y `person_segments` correspondientes en `analytics.db`.
- Se purgan inmediatamente todos los `CameraEvent` asociados al MKV eliminado, tanto en borrado individual como masivo.
- Además, `GET /api/events` verifica que el archivo de cada evento grabado siga existiendo y descarta/purga referencias huérfanas, cubriendo borrados externos o inconsistencias antiguas.
- El frontend llama a `refreshEvents(false)` tras un borrado para retirar las tarjetas sin esperar al siguiente polling.
- Si un job NanoDet estaba analizando un archivo borrado, detecta que ya no existe y evita recrear metadata huérfana.
- El reproductor tiene botón **Eliminar** con modal de confirmación.
- Retención por día seleccionado, semana, mes, mes anterior, rango personalizado y todas; botón dedicado **Eliminar todos** conserva cualquier grabación activa.

## Pruebas y validación
- Tests de duración exacta+caché, rechazo de borrado activo, borrado cerrado y limpieza de analítica.
- Tests de codificación JPEG de frame `person`, espera de frame reciente, `fitWithin` 16:9/4:3 y letterbox 320x320; comprueba que el JPEG notificable 16:9 termina 320x180, no 1:1.
- Tests de borrado HTTP por rango, purga selectiva de eventos y filtrado de eventos cuyo MKV ya no existe.
- `go test ./...` pasa.
- `go vet ./...` pasa.
- Build principal `CGO_ENABLED=1` pasa.
- Build de compatibilidad `CGO_ENABLED=0` pasa.
- `sh -n install.sh` pasa.
- JavaScript validado con esbuild.
- Auditoría DOM final: 155 IDs HTML, 148 referencias JS exactas, 0 faltantes.
- `git diff --check` pasa.
- `install.sh` se ejecutó realmente y confirmó idempotencia del toolchain/ONNX Runtime.
- Servicio OpenRC activo y `/api/health` responde OK.
- SHA-256 de `dist/onvif-visor` y `/opt/onvif-visor/onvif-visor` coincide: `00a2fe58b1a8fb2eff6ef3ffa0798bc651dc9e4b99b1d2a6f49511e83a7729bf`.
- `/api/events` continúa protegido: HTTP 401 sin sesión.
- Smoke temporal previo validó metadata exacta, DELETE individual y borrado por rango sin tocar grabaciones productivas.
- Una grabación real `20260912-202851.250.mkv` reportó duración exacta `20.160 s` en la prueba de metadata.

## Límite de verificación
El host no dispone de navegador/CDP, por lo que la apariencia final del bloque de retención, selector Telegram y miniaturas no pudo inspeccionarse pixel a pixel. Sintaxis JS, DOM, CSS estructural, APIs, tests backend y servicio desplegado sí se validaron.

## Resultado
Funcionalidad completada, desplegada y lista para uso.
