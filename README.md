# ONVIF Visor

Visor local de cámaras ONVIF escrito en Go. Descubre dispositivos en la red, permite agregarlos con credenciales, muestra previews y transmisiones en vivo, ofrece multivista adaptable y organiza las grabaciones disparadas por eventos ONVIF en una biblioteca con línea de tiempo.

## Características

- Descubrimiento WS-Discovery en todas las interfaces activas con multicast, hasta 16 interfaces.
- Alta de cámara desde el descubrimiento o mediante endpoint ONVIF manual.
- Validación de credenciales y lectura de fabricante, modelo, firmware y perfiles ONVIF.
- Selección automática del perfil con mayor resolución disponible.
- URI RTSP persistida sin usuario ni contraseña.
- Contraseñas de cámaras cifradas localmente con AES-256-GCM.
- Interfaz web profesional embebida dentro del binario Go.
- Login propio del frontend con sesión `HttpOnly` cuando el visor se publica en LAN.
- Galería inicial tipo thumbnails: cada cámara obtiene un snapshot JPEG bajo demanda en vez de abrir un stream continuo.
- Click en una miniatura para abrir la transmisión RTSP -> MJPEG en tiempo real.
- Multivista adaptable de hasta 9 cámaras: 2 mitad/mitad, 3 con vista principal + dos secundarias, 4 en 2x2, 5-6 en 3x2 y 7-9 en 3x3.
- Selector de cámaras dentro de la multivista para agregar o quitar streams mientras se monitorea.
- Suscripción ONVIF PullPoint para recibir eventos.
- Grabación automática ante eventos de movimiento, tamper, entrada digital o alarma.
- Post-roll configurable de 1 a 600 segundos.
- Grabación con `-c copy` a MKV para evitar recodificación siempre que el stream sea compatible.
- Biblioteca de grabaciones por cámara con cantidad, espacio usado y duración aproximada.
- Miniaturas JPEG de grabaciones.
- Línea de tiempo de 24 horas filtrable por fecha.
- Reproducción web de clips mediante FFmpeg -> MJPEG, con pausa/reanudación y seek por segundos.
- Descarga autenticada del MKV original.
- Servicio ligado a `127.0.0.1` por defecto.
- Sin base de datos: configuración en archivos locales.

## Arquitectura

```text
cmd/onvif-visor/      ejecutable principal
internal/onvif/       descubrimiento, perfiles, RTSP y Events/PullPoint
internal/store/       persistencia y cifrado de credenciales
internal/media/       FFmpeg live, snapshots, grabación y playback
internal/server/      API HTTP, seguridad y coordinación
web/                  HTML/CSS/JS embebidos con go:embed
contexto/             bitácora Ponytail
tareas/               tareas Ponytail
```

El binario de la aplicación se compila con `CGO_ENABLED=0`, por lo que es estático. FFmpeg es una dependencia de runtime independiente y se usa únicamente para medios: RTSP -> MJPEG, snapshots, escritura de grabaciones y reproducción web de clips. No se requiere Node.js, Python, PHP, nginx ni una base de datos.

## Requisitos

- Go 1.24 o superior para compilar. El proyecto fue validado con Go 1.27.1.
- FFmpeg accesible en `PATH` o indicado con `-ffmpeg`.
- Cámara ONVIF accesible desde la misma red del host.
- Para grabación por eventos, la cámara debe exponer ONVIF Events/PullPoint y tener habilitado en su firmware el evento que se desea reportar.

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
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor
```

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
recordings/<camera-id>/*.mkv
```

`master.key`, configuración sensible y grabaciones están ignorados por Git.

## Publicar en la LAN

No se permite escuchar fuera de loopback sin usuario y contraseña. Ejemplo:

```bash
ONVIF_VISOR_USER='operador' \
ONVIF_VISOR_PASSWORD='cambia-esto-por-una-clave-larga' \
  ./dist/onvif-visor -listen 0.0.0.0:8080
```

La propia interfaz muestra una pantalla de login. Al validar las credenciales, el servidor crea una cookie de sesión `HttpOnly` y `SameSite=Strict`; no se usan diálogos HTTP Basic, `window.prompt` ni credenciales incrustadas en URLs.

Para una red no confiable se recomienda colocar TLS delante del servicio. El modo LAN integrado usa HTTP y está pensado para una LAN privada/controlada.

## Flujo de uso

1. Iniciar sesión si el visor está publicado en LAN.
2. La vista **Cámaras** muestra todas las cámaras como miniaturas. El thumbnail se genera mediante un snapshot ligero, no abriendo un live permanente.
3. Pulsar una cámara para abrirla en **Vista en vivo**.
4. Usar **Agregar cámaras** dentro del monitor para activar multivista. El layout se reorganiza automáticamente hasta un máximo de 9 streams.
5. Usar **Detectar cámaras** o **Agregar cámara** para incorporar nuevos endpoints ONVIF.
6. Si una cámara soporta Events, activar **Grabar eventos** desde su tarjeta.
7. Abrir **Grabaciones** para elegir una cámara, revisar sus clips, filtrar la fecha y usar la línea de tiempo.
8. Pulsar una tarjeta o bloque de timeline para reproducir. El scrubber reinicia la reproducción desde el segundo seleccionado y el MKV original puede descargarse.
9. Abrir **Eventos** para consultar la actividad ONVIF reciente y saber cuáles dispararon grabación.

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

La hora de inicio se obtiene del nombre que genera el grabador (`YYYYMMDD-HHMMSS.mmm.mkv`). Para mantener una sola dependencia externa, la duración mostrada se aproxima usando el timestamp de inicio y la última modificación del archivo; no se requiere `ffprobe` adicional.

El playback del navegador convierte el MKV a MJPEG a velocidad real. Permite seek por segundos, pausa/reanudación y hasta 4 reproducciones simultáneas. El playback web actual no incluye audio; la descarga conserva el MKV original.

## Eventos que disparan grabación

El visor reconoce tópicos/datos relacionados con:

- motion / cell motion
- tamper
- digital input
- alarm

Cuando un evento incluye un estado explícito `false`, `off`, `inactive`, `idle` o equivalente, no se inicia una nueva grabación. Si llega otro evento activo durante una grabación, el vencimiento del post-roll se extiende en vez de crear otro proceso FFmpeg para la misma cámara.

## Seguridad

- La contraseña de la cámara no se devuelve en las APIs de listado.
- Las credenciales se cifran con AES-256-GCM antes de persistirse.
- El archivo `master.key` se crea con permisos `0600` y el directorio de datos con `0700`.
- El URI RTSP guardado no contiene credenciales.
- La salida de FFmpeg se descarta para evitar que errores puedan terminar registrando un URI RTSP sensible.
- El servicio escucha únicamente en loopback salvo que se solicite explícitamente LAN y se configuren usuario y contraseña.
- La sesión LAN usa un token aleatorio en cookie `HttpOnly`, `SameSite=Strict` y `Secure` cuando la petición entra por HTTPS.
- Las rutas de grabaciones validan que el archivo solicitado sea un basename `.mkv` dentro del directorio calculado de la cámara; se rechaza path traversal.
- Los endpoints de snapshot, live, playback y descarga permanecen detrás de la misma sesión de UI.

Nota: FFmpeg necesita recibir credenciales RTSP para autenticar contra determinadas cámaras. Se entregan a FFmpeg únicamente en memoria al lanzar el proceso; no se escriben en archivos de configuración ni en logs del visor. En hosts multiusuario, los argumentos de procesos pueden ser visibles para usuarios con privilegios suficientes; se recomienda ejecutar el visor con una cuenta de servicio dedicada.

## Limitaciones conocidas

- MJPEG prioriza simplicidad y compatibilidad del navegador. Consume más ancho de banda/CPU que WebRTC/HLS.
- El live está limitado a 10 fps y 960 px de ancho por stream.
- Se permiten hasta 4 espectadores live simultáneos por cámara y hasta 4 playbacks de grabaciones simultáneos.
- Una multivista de 9 cámaras implica 9 procesos FFmpeg live; la capacidad real depende de CPU, red y resolución de origen.
- El descubrimiento depende de multicast UDP/3702; VLAN, firewalls o routers que bloqueen WS-Discovery pueden impedir encontrar una cámara aunque el alta manual siga funcionando.
- Algunos fabricantes anuncian ONVIF Events pero implementan PullPoint de forma incompleta. El visor reintenta la suscripción; la disponibilidad real depende del firmware.
- La grabación usa Matroska (`.mkv`) y copia los codecs del origen. Si una cámara entrega un formato que FFmpeg no puede remuxear a MKV, la grabación fallará y requerirá una política específica.
- Un clip todavía activo puede no ser reproducible hasta que FFmpeg cierre correctamente el contenedor MKV.
- Si se elimina una cámara, sus grabaciones permanecen físicamente en disco pero dejan de aparecer en la biblioteca mientras esa cámara no esté configurada.
- No se implementan todavía PTZ, analítica propia, nube ni detección por IA.

## Flags

```text
-listen             dirección HTTP; default 127.0.0.1:8080
-data-dir           directorio de configuración; default data
-recordings-dir     directorio de grabaciones; default recordings
-ffmpeg             ruta de ffmpeg; default ffmpeg
-ui-user            usuario para modo LAN; también ONVIF_VISOR_USER
-ui-password        contraseña para modo LAN; también ONVIF_VISOR_PASSWORD
```

## Validación del proyecto

Antes de una entrega se deben ejecutar:

```bash
gofmt -w cmd internal web
go test ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor
```
