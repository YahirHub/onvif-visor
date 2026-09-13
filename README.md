# ONVIF Visor

Visor local de cámaras ONVIF escrito en Go. Descubre dispositivos en la red, permite agregarlos con credenciales, muestra previews y transmisiones en vivo, ofrece multivista adaptable, organiza grabaciones por eventos u horario y añade analítica local de personas sobre el archivo de vídeo.

## Características

- Descubrimiento WS-Discovery en todas las interfaces activas con multicast, hasta 16 interfaces.
- Alta de cámara desde el descubrimiento o mediante endpoint ONVIF manual.
- Validación de credenciales y lectura de fabricante, modelo, firmware y perfiles ONVIF.
- Selección automática del perfil con mayor resolución disponible.
- URI RTSP persistida sin usuario ni contraseña.
- Contraseñas de cámaras cifradas localmente con AES-256-GCM.
- Interfaz web profesional embebida dentro del binario Go.
- Login propio del frontend respaldado por SQLite embebido: contraseñas bcrypt y sesiones `HttpOnly` persistentes de 30 días que sobreviven reinicios.
- Vista **Sesiones** con IP/dispositivo/actividad y revocación individual o cierre de todas las demás sesiones conservando la actual.
- Vista **Perfil** para cambiar usuario y contraseña; exige la clave actual y al cambiarla revoca automáticamente las demás sesiones.
- Galería inicial tipo thumbnails: cada cámara obtiene un snapshot JPEG bajo demanda en vez de abrir un stream continuo.
- Click en una miniatura para abrir la transmisión RTSP -> MJPEG en tiempo real con cola de un solo frame: si el cliente se atrasa se descartan cuadros viejos en vez de acumular delay.
- Multivista adaptable de hasta 9 cámaras: 2 mitad/mitad, 3 con vista principal + dos secundarias, 4 en 2x2, 5-6 en 3x2 y 7-9 en 3x3.
- Selector de cámaras dentro de la multivista para agregar o quitar streams sin reiniciar los que ya están reproduciéndose; el perfil ligero mantiene el mismo RTSP y reduce FPS/resolución sin cambiar la relación de aspecto.
- Botón de pantalla completa para el monitor: Fullscreen API cuando está disponible y modo teatro sin sidebar como fallback.
- Diseño responsivo para desktop, tablet y móvil, con safe-area, navegación inferior desplazable y panes 16:9 en pantallas pequeñas.
- Skeletons de carga para cámaras, grabaciones, eventos, sesiones, descubrimiento e Integraciones/Telegram, con `prefers-reduced-motion` para evitar animaciones cuando el sistema lo solicita.
- Suscripción ONVIF PullPoint para recibir eventos cuando el firmware la implementa correctamente.
- Detector local de movimiento por FFmpeg como respaldo cuando PullPoint falla o no está disponible.
- Grabación automática ante movimiento/eventos y grabación programada por días y horario.
- Post-roll configurable de 1 a 600 segundos.
- Grabación con `-c copy` a MKV para evitar recodificación siempre que el stream sea compatible.
- Biblioteca de grabaciones por cámara con cantidad, espacio usado y duración aproximada.
- Miniaturas JPEG de grabaciones y eventos; si NanoDet encontró una persona, se usa preferentemente el frame de mayor confianza para que pueda reconocerse sin abrir el clip. Un evento asociado a un clip puede abrir la reproducción directamente en su segundo exacto.
- Línea de tiempo de 24 horas filtrable por fecha; al reproducir un clip cerrado se consulta y cachea su duración real con `ffprobe` para que scrubber y contador terminen junto con el video.
- Eliminación segura de grabaciones con modal: clip individual, día, semana, mes, mes anterior, rango personalizado o todos los clips cerrados de una cámara; nunca borra el archivo que sigue grabándose.
- Analítica local de personas con **NanoDet-Plus-m 320** sobre ONNX Runtime: muestreo base de 1 FPS, una sola inferencia a la vez y sin enviar vídeo a servicios externos.
- Backfill automático de grabaciones cerradas y detección incremental durante grabaciones activas/programadas; los resultados se guardan en `data/analytics.db`.
- Pista **Personas** bajo la línea de tiempo: cada tramo detectado es clicable y abre el MKV en el segundo de mayor confianza de esa detección.
- Escritura Matroska con clusters cortos, flush frecuente y sincronización periódica a disco para reducir el daño ante una interrupción brusca.
- Compresión H.264 en segundo plano al cerrar un clip; el original sólo se reemplaza atómicamente cuando el resultado es válido y realmente ahorra espacio.
- Módulo **Integraciones > Telegram**: valida el token con Bot API, lo guarda cifrado, vincula chats con códigos de 4 dígitos de un solo uso válidos 10 minutos y permite eliminar por completo la integración.
- Alertas Telegram configurables por evento con foto o fragmento MP4 de 3–60 s; en modo foto sólo se envía un frame que NanoDet haya confirmado como persona. También incluye `/camaras`, `/foto <camara>`, `/video <camara>` y `/video <camara> <segundos>` para chats vinculados.
- Reproducción web de clips mediante FFmpeg -> MJPEG, con pausa/reanudación y seek por segundos.
- Descarga autenticada del MKV original.
- Servicio ligado a `127.0.0.1` por defecto.
- Sin servidor de base de datos externo: cámaras permanecen en archivos locales y usuarios/sesiones/integraciones usan SQLite embebido dentro de `data/`.

## Arquitectura

```text
cmd/onvif-visor/      ejecutable principal
internal/onvif/       descubrimiento, perfiles, RTSP y Events/PullPoint
internal/store/       persistencia de cámaras y cifrado AES-GCM de secretos
internal/account/     SQLite embebido para usuarios, sesiones y estado Telegram
internal/analytics/   NanoDet/ONNX Runtime, indexación 1 FPS y analytics.db
internal/telegram/    cliente Bot API con long polling y envío de medios
internal/media/       FFmpeg live, snapshots, grabación, clips y playback
internal/server/      API HTTP, seguridad, sesiones, eventos, analítica e integraciones
internal/service/     instalación en /opt y gestores systemd/OpenRC/runit/SysV
web/                  HTML/CSS/JS embebidos con go:embed
contexto/             bitácora Ponytail
tareas/               tareas Ponytail
```

La compilación principal usa `CGO_ENABLED=1` porque la analítica carga ONNX Runtime mediante su librería compartida. SQLite sigue siendo una implementación pura Go (`modernc.org/sqlite`), sin servicio externo. FFmpeg se usa para medios y para extraer frames 320x320 a 1 FPS; NanoDet hace la inferencia en CPU dentro del proceso del visor. Existe un build estático opcional con `CGO_ENABLED=0`, pero en ese binario la analítica NanoDet queda deshabilitada de forma segura. No se requiere Node.js, Python, PHP, nginx ni un servidor de base de datos.

## Requisitos

- Go 1.24 o superior para compilar. El proyecto fue validado con Go 1.27.1.
- FFmpeg accesible en `PATH` o indicado con `-ffmpeg`.
- ONNX Runtime 1.x disponible como `libonnxruntime.so`/`libonnxruntime.so.1` para habilitar NanoDet. En el servidor DexOS/Alpine validado: `apk add --no-cache onnxruntime`.
- Cámara ONVIF accesible desde la misma red del host.
- Para eventos nativos se usa ONVIF Events/PullPoint; si la cámara lo anuncia pero falla, el visor puede usar un detector local de movimiento sobre RTSP como respaldo.

## Compilar

```bash
make test
make build
```

El resultado queda en:

```text
dist/onvif-visor
```

La compilación equivalente sin Make es:

```bash
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor
```

Para generar además un binario estático sin inferencia NanoDet:

```bash
make build-static
```

## Instalar como servicio

La ruta recomendada desde el código fuente es `install.sh`. Detecta `apk`, `apt`, `dnf`, `yum`, `pacman` o `zypper`; instala compilador C, Go, Make, Git, curl, FFmpeg/ffprobe y certificados. Si el Go del sistema es anterior a 1.24 descarga una versión estable oficial desde `go.dev`. Para ONNX Runtime primero intenta el paquete nativo del sistema —obligatorio en musl/Alpine— y, en Linux glibc x86_64/aarch64, puede descargar automáticamente el release oficial más reciente de Microsoft. Después ejecuta `go test`, `go vet`, `make build` y finalmente `onvif-visor --install`.

```bash
./install.sh
```

Los flags adicionales se pasan al instalador del visor; por ejemplo, para el primer usuario de una instalación nueva:

```bash
./install.sh -ui-user operador -ui-password 'una-clave-larga'
```

Para omitir únicamente las pruebas automáticas durante una actualización controlada puede usarse `ONVIF_SKIP_TESTS=1 ./install.sh`. No se recomienda en una instalación normal.

El instalador interno detecta automáticamente `systemd`, OpenRC, runit o SysV init, eleva privilegios con `sudo`, `doas` o `pkexec` cuando hace falta e instala el binario en `/opt/onvif-visor/onvif-visor`. También materializa el modelo NanoDet embebido en `/opt/onvif-visor/models/`. En la primera instalación migra `data/` y `recordings/` desde `--source-dir` sólo si los destinos están vacíos; en actualizaciones nunca sobreescribe esos datos. ONNX Runtime sigue siendo una dependencia del sistema; el visor lo autodetecta en rutas estándar o `install.sh` persiste explícitamente la ruta encontrada.

La instalación manual sigue disponible:

```bash
sudo ./dist/onvif-visor --install --listen 0.0.0.0:8081 --source-dir .
```

También crea `/usr/local/bin/onvif-visor`. El servicio puede administrarse sin conocer qué init system usa el host:

```bash
onvif-visor --service status
sudo onvif-visor --service restart
sudo onvif-visor --service stop
sudo onvif-visor --service start
```

Para desinstalar el servicio y el binario, conservando cámaras, SQLite y grabaciones en `/opt/onvif-visor/`, usar:

```bash
sudo onvif-visor --remove
```

La configuración de instalación no contiene secretos: `install.json` guarda únicamente el gestor detectado, dirección de escucha y rutas de FFmpeg/ONNX Runtime. Las cuentas permanecen en SQLite y los secretos de cámaras/Telegram siguen protegidos por `master.key`.

## Ejecutar localmente

```bash
./dist/onvif-visor
```

Abrir:

```text
http://127.0.0.1:8080
```

Por defecto se crean:

```text
data/master.key
data/cameras.json
data/accounts.db
data/analytics.db
data/models/nanodet-plus-m_320.onnx
recordings/<camera-id>/*.mkv
```

`master.key`, configuración sensible y grabaciones están ignorados por Git.

## Publicar en la LAN

No se permite escuchar fuera de loopback sin que exista al menos un usuario en SQLite. En el primer arranque LAN se crea/actualiza el usuario bootstrap con los flags o variables de entorno; después la cuenta permanece en `data/accounts.db` y no es necesario volver a pasar las credenciales para conservarla. Ejemplo:

```bash
ONVIF_VISOR_USER='operador' \
ONVIF_VISOR_PASSWORD='cambia-esto-por-una-clave-larga' \
  ./dist/onvif-visor -listen 0.0.0.0:8080
```

La propia interfaz muestra una pantalla de login. Al validar las credenciales contra bcrypt, el servidor crea una cookie `HttpOnly` y `SameSite=Strict` con un token aleatorio; SQLite conserva únicamente su SHA-256. La sesión dura 30 días, sobrevive reinicios y puede revocarse desde **Sesiones**. No se usan diálogos HTTP Basic, `window.prompt` ni credenciales incrustadas en URLs.

Para una red no confiable se recomienda colocar TLS delante del servicio. El modo LAN integrado usa HTTP y está pensado para una LAN privada/controlada.

## Flujo de uso

1. Iniciar sesión si el visor está publicado en LAN.
2. La vista **Cámaras** muestra todas las cámaras como miniaturas. El thumbnail se genera mediante un snapshot ligero, no abriendo un live permanente.
3. Pulsar una cámara para abrirla en **Vista en vivo**.
4. Usar **Agregar cámaras** dentro del monitor para activar multivista. El layout se reorganiza automáticamente hasta un máximo de 9 streams.
5. Usar **Detectar cámaras** o **Agregar cámara** para incorporar nuevos endpoints ONVIF.
6. Activar **REC** en una tarjeta para grabar cuando haya movimiento/eventos.
7. Usar el botón **◷** de la tarjeta para activar un horario, elegir días de la semana y definir hora inicial/final; los horarios que cruzan medianoche están soportados.
8. Abrir **Grabaciones** para elegir una cámara, revisar sus clips, filtrar la fecha y usar la línea de tiempo. La pista **Personas** muestra los tramos que NanoDet va encontrando tanto en clips históricos como durante grabaciones activas.
9. Pulsar una tarjeta, un bloque de grabación o un tramo de **Personas** para reproducir. Una detección de persona salta al segundo de mayor confianza del tramo; el scrubber permite buscar otro instante y el MKV original puede descargarse.
10. Abrir **Eventos** para consultar movimiento y eventos recientes y saltar al instante de una grabación asociada.
11. Abrir **Sesiones** para revisar accesos persistentes y cerrar todos los demás dispositivos sin perder la sesión actual.
12. Abrir **Perfil** para cambiar el usuario o contraseña; la clave actual confirma la operación y una clave nueva cierra las demás sesiones.
13. Abrir **Integraciones > Telegram** para validar el token, vincular chats y elegir alertas por foto o vídeo y su duración por defecto.
14. En **Multivista**, usar **⛶** para fullscreen; en navegadores sin Fullscreen API se activa el modo teatro sin sidebar.

## Endpoint manual

Si solo se introduce una IP o `IP:puerto`, el visor completa automáticamente:

```text
http://IP:puerto/onvif/device_service
```

También se puede indicar directamente un endpoint HTTPS/HTTP ONVIF válido.

## Multivista

La multivista prioriza una superficie grande y legible sin mantener streams activos en la galería principal:

```text
1 cámara    1x1
2 cámaras   1x2, mitad y mitad
3 cámaras   principal a la izquierda + 2 secundarias apiladas
4 cámaras   2x2
5-6         3x2
7-9         3x3
móvil       cámaras apiladas verticalmente
```

Cada stream sigue usando el pipeline MJPEG del visor. El selector lateral permite agregar o quitar cámaras mientras la multivista está abierta.

## Biblioteca y línea de tiempo

Las grabaciones se almacenan en `recordings/<camera-id>/` y solo se indexan archivos regulares `.mkv`. La API no devuelve la ruta física del host: identifica cada clip por `camera_id` + nombre de archivo validado.

La hora de inicio se obtiene del nombre que genera el grabador (`YYYYMMDD-HHMMSS.mmm.mkv`). El listado inicial conserva una estimación barata para no lanzar decenas de procesos sobre bibliotecas grandes, pero al abrir un clip el servidor obtiene su duración real con `ffprobe` y la cachea por tamaño+mtime. El scrubber, contador y final de reproducción usan ese valor exacto, por lo que el reloj deja de avanzar cuando termina el contenido real del MKV.

El playback del navegador convierte el MKV almacenado a MJPEG a velocidad real. Permite seek por segundos, pausa/reanudación y hasta 4 reproducciones simultáneas. El playback web actual no incluye audio; la descarga entrega el MKV que quedó almacenado después de la optimización de espacio. Desde el reproductor se puede eliminar el clip actual con confirmación. La barra de retención permite eliminar por día seleccionado, semana, mes, mes anterior, rango personalizado o todas las grabaciones cerradas de la cámara; una grabación activa nunca se elimina. Al borrar un clip también se purgan su metadata NanoDet y los eventos en memoria asociados, y `/api/events` filtra cualquier referencia cuyo MKV ya no exista.

La analítica usa el modelo oficial NanoDet-Plus-m 320 y únicamente consume la clase COCO `person`. El modelo y su licencia Apache-2.0 se conservan en `internal/analytics/assets/` y el instalador despliega ambos junto al servicio. Para clips cerrados, un único worker procesa frames a 1 FPS y guarda intervalos fusionados en `analytics.db`; la huella tamaño+mtime evita repetir archivos ya terminados. Durante una grabación activa se analiza el substream ligero cuando está disponible y se registra el offset contra el MKV que se está escribiendo, por lo que una grabación programada de varias horas puede mostrar detecciones antes de cerrarse. El botón **Reanalizar** marca la biblioteca como pendiente y el scanner la vuelve a recorrer por tandas sin duplicar trabajos ya en cola.

## Eventos que disparan grabación

El visor reconoce tópicos/datos relacionados con:

- motion / cell motion
- tamper
- digital input
- alarm

Cuando un evento incluye un estado explícito `false`, `off`, `inactive`, `idle` o equivalente, no se inicia una nueva grabación. Si llega otro evento activo durante una grabación, el vencimiento del post-roll se extiende en vez de crear otro proceso FFmpeg para la misma cámara.

## Integración Telegram

La vista **Integraciones** permite conectar un bot dedicado mediante el token de BotFather. El backend valida el token con `getMe`, cifra el secreto con la misma llave maestra AES-GCM usada para otros secretos y nunca lo devuelve a la API del navegador. Una vez configurado se muestra la URL `https://t.me/<bot>` y pueden generarse códigos de 4 dígitos de un solo uso, válidos 10 minutos. En Telegram se aceptan `/start CODIGO` y `/login CODIGO`.

Un chat vinculado puede usar `/camaras`, `/foto <camara>`, `/video <camara>` y `/video <camara> <segundos>`. `/video <camara>` usa la duración de vídeo por defecto configurada en el panel y `/video Cocina 30` puede sobrescribirla entre 3 y 60 segundos. El selector **Medio de las alertas automáticas** sólo decide qué adjuntan las alertas; no modifica el comportamiento de `/foto` ni `/video`, y la duración permanece editable aunque las alertas estén en modo foto. Los clips se recodifican a H.264 1280px/12 fps con timestamps reiniciados, warm-up inicial y bitrate acotado para evitar los fragmentos de ~1 segundo y el efecto borroso observado al arrancar directamente desde RTSP. Para no bloquear el bot ni sobrecargar el host, el long polling continúa mientras se genera el clip y sólo se permite una captura Telegram simultánea. Cuando las alertas automáticas están configuradas como **foto**, ya no se envía el primer frame genérico del evento: el backend espera hasta 10 segundos un frame que NanoDet haya clasificado realmente como `person`, conserva la relación de aspecto original del substream y envía exactamente esa captura con su confianza; si no aparece ninguna persona, no genera una alerta fotográfica falsa. `/foto <camara>` sigue siendo una captura manual inmediata. Las alertas en modo vídeo conservan la duración configurada. Eliminar la integración borra el token cifrado, los códigos y los chats vinculados.

## Seguridad

- La contraseña de la cámara no se devuelve en las APIs de listado.
- Las credenciales se cifran con AES-256-GCM antes de persistirse.
- El archivo `master.key` se crea con permisos `0600` y el directorio de datos con `0700`.
- El URI RTSP guardado no contiene credenciales.
- La salida de FFmpeg se descarta para evitar que errores puedan terminar registrando un URI RTSP sensible.
- El servicio escucha únicamente en loopback salvo que se solicite explícitamente LAN y exista al menos un usuario persistido; las credenciales bootstrap sólo son obligatorias para crear la primera cuenta.
- Las contraseñas de usuarios se almacenan con bcrypt. La sesión usa un token aleatorio en cookie `HttpOnly`, `SameSite=Strict` y `Secure` cuando la petición entra por HTTPS; SQLite persiste únicamente el SHA-256 del token.
- Los tokens de Telegram se validan antes de guardarse, se cifran con AES-256-GCM y no se devuelven a las APIs de configuración.
- `analytics.db` se crea con permisos `0600`; sólo guarda metadatos de detección (archivo, segundos y confianza), no frames ni imágenes extraídas.
- Las rutas de grabaciones validan que el archivo solicitado sea un basename `.mkv` dentro del directorio calculado de la cámara; se rechaza path traversal.
- Los endpoints de snapshot, live, playback y descarga permanecen detrás de la misma sesión de UI.

Nota: FFmpeg necesita recibir credenciales RTSP para autenticar contra determinadas cámaras. Se entregan a FFmpeg únicamente en memoria al lanzar el proceso; no se escriben en archivos de configuración ni en logs del visor. En hosts multiusuario, los argumentos de procesos pueden ser visibles para usuarios con privilegios suficientes; se recomienda ejecutar el visor con una cuenta de servicio dedicada.

## Limitaciones conocidas

- MJPEG prioriza simplicidad y compatibilidad del navegador. Consume más ancho de banda/CPU que WebRTC/HLS.
- La vista individual usa hasta 8 fps y 960 px; las nuevas cámaras añadidas a una multivista usan hasta 5 fps y 640 px para reducir CPU, red y latencia.
- Se permiten hasta 4 espectadores live simultáneos por cámara y hasta 4 playbacks de grabaciones simultáneos.
- Una multivista de 9 cámaras implica 9 procesos FFmpeg live; la capacidad real depende de CPU, red y resolución de origen.
- El descubrimiento depende de multicast UDP/3702; VLAN, firewalls o routers que bloqueen WS-Discovery pueden impedir encontrar una cámara aunque el alta manual siga funcionando.
- Algunos fabricantes anuncian ONVIF Events pero implementan PullPoint de forma incompleta. En ese caso el visor cambia automáticamente al detector local de movimiento por FFmpeg; esto añade un proceso ligero adicional por cámara afectada.
- La grabación usa Matroska (`.mkv`) y copia los codecs del origen. Si una cámara entrega un formato que FFmpeg no puede remuxear a MKV, la grabación fallará y requerirá una política específica.
- Un clip todavía activo puede no ser reproducible hasta que FFmpeg cierre correctamente el contenedor MKV.
- Si se elimina una cámara, sus grabaciones permanecen físicamente en disco pero dejan de aparecer en la biblioteca mientras esa cámara no esté configurada.
- NanoDet trabaja a 1 FPS para mantener bajo el consumo de CPU; una aparición de menos de un segundo puede quedar entre muestras. La analítica actual sólo clasifica `person`: no hace reconocimiento facial, identidad, tracking persistente ni análisis en la nube. Por la misma razón, una alerta Telegram configurada como foto se omite si no hay ningún frame `person` durante la ventana de espera de 10 segundos.
- Si `libonnxruntime` no está disponible, el visor sigue funcionando pero reporta la pista de personas como no disponible. El build estático `CGO_ENABLED=0` tiene el mismo comportamiento degradado.
- No se implementan todavía PTZ ni nube.

## Flags

```text
-listen             dirección HTTP; default 127.0.0.1:8080
-data-dir           directorio de configuración; default data
-recordings-dir     directorio de grabaciones; default recordings
-ffmpeg             ruta de ffmpeg; default ffmpeg
-analytics-model    ruta opcional al modelo NanoDet ONNX; por defecto usa el embebido
-onnxruntime        ruta opcional a libonnxruntime.so; también ONVIF_ONNXRUNTIME
-ui-user            usuario bootstrap; también ONVIF_VISOR_USER
-ui-password        contraseña bootstrap; también ONVIF_VISOR_PASSWORD
-install            instalar/actualizar como servicio en /opt/onvif-visor
-remove             retirar servicio/binario conservando data y recordings
-service            status|start|stop|restart del servicio instalado
-source-dir         origen de data/ y recordings/ para la primera instalación
```

## Validación del proyecto

Antes de una entrega se deben ejecutar:

```bash
gofmt -w cmd internal web
go test ./...
go vet ./...
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/onvif-visor-static ./cmd/onvif-visor
```
