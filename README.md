# ONVIF Visor

Visor local de cámaras ONVIF escrito en Go. Descubre dispositivos en la red, permite agregarlos con credenciales, consulta sus perfiles de vídeo, muestra el stream en vivo en el navegador y puede iniciar grabaciones cuando la cámara reporta eventos ONVIF de movimiento/alarma.

## Características

- Descubrimiento WS-Discovery en todas las interfaces activas con multicast (hasta 16 interfaces).
- Alta de cámara desde el descubrimiento o mediante endpoint ONVIF manual.
- Validación de credenciales y lectura de fabricante, modelo, firmware y perfiles ONVIF.
- Selección automática del perfil con mayor resolución disponible.
- URI RTSP persistida sin usuario ni contraseña.
- Contraseñas cifradas localmente con AES-256-GCM.
- Interfaz web embebida dentro del binario Go.
- Vista en vivo RTSP -> MJPEG mediante FFmpeg, sin servidor web adicional.
- Suscripción ONVIF PullPoint para recibir eventos.
- Grabación automática ante eventos de movimiento, tamper, entrada digital o alarma.
- Post-roll configurable de 1 a 600 segundos.
- Grabación con `-c copy` a MKV para evitar recodificación siempre que el stream sea compatible.
- Servicio ligado a `127.0.0.1` por defecto.
- Cuando se publica en LAN, exige token de acceso.
- Sin base de datos: configuración en archivos locales.

## Arquitectura

```text
cmd/onvif-visor/      ejecutable principal
internal/onvif/       descubrimiento, perfiles, RTSP y Events/PullPoint
internal/store/       persistencia y cifrado de credenciales
internal/media/       procesos FFmpeg para live y grabación
internal/server/      API HTTP, seguridad y coordinación
web/                  HTML/CSS/JS embebidos con go:embed
contexto/             bitácora Ponytail
tareas/               tareas Ponytail
```

El binario de la aplicación se compila con `CGO_ENABLED=0`, por lo que es estático. FFmpeg es una dependencia de runtime independiente y se usa únicamente para transportar/transcodificar el vídeo hacia el navegador y para escribir las grabaciones. No se requiere Node.js, Python, PHP, nginx ni una base de datos.

## Requisitos

- Go 1.24 o superior para compilar. El proyecto fue validado con Go 1.27.1.
- FFmpeg accesible en `PATH` o indicado con `-ffmpeg`.
- Cámara ONVIF accesible desde la misma red del host.
- Para grabación por eventos, la cámara debe exponer ONVIF Events/PullPoint y tener habilitado en su firmware el evento que se desea reportar.

En el host donde se desarrolló este proyecto FFmpeg ya quedó instalado.

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

No se permite escuchar fuera de loopback sin token. Ejemplo:

```bash
ONVIF_VISOR_TOKEN='cambia-esto-por-un-token-largo' \
  ./dist/onvif-visor -listen 0.0.0.0:8080
```

La interfaz solicitará el token al primer acceso a la API y lo guardará en una cookie `HttpOnly` de la sesión local del navegador.

Para una red no confiable se recomienda colocar TLS delante del servicio. El modo LAN integrado usa HTTP y está pensado para una LAN privada/controlada.

## Flujo de uso

1. Abrir el visor.
2. Pulsar **Detectar cámaras**.
3. Elegir un dispositivo descubierto y pulsar **Agregar**, o usar **Agregar cámara** para introducir el endpoint manualmente.
4. Escribir usuario y contraseña de la cámara.
5. El servidor valida el dispositivo, obtiene sus perfiles y guarda la configuración.
6. Pulsar **Ver en vivo** para abrir el stream.
7. Si la cámara soporta ONVIF Events, activar **Grabar por eventos** y ajustar el post-roll.
8. Los eventos recientes aparecerán en el panel inferior. Los que activen el disparador se marcan como eventos de grabación.

## Endpoint manual

Si solo se introduce una IP o `IP:puerto`, el visor completa automáticamente:

```text
http://IP:puerto/onvif/device_service
```

También se puede indicar directamente un endpoint HTTPS/HTTP ONVIF válido.

## Eventos que disparan grabación

El MVP reconoce tópicos/datos relacionados con:

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
- El servicio escucha únicamente en loopback salvo que se solicite explícitamente LAN y se configure un token.

Nota: FFmpeg necesita recibir credenciales RTSP para autenticar contra determinadas cámaras. En este MVP se entregan a FFmpeg únicamente en memoria al lanzar el proceso; no se escriben en los archivos de configuración ni en los logs del visor. En hosts multiusuario, los argumentos de procesos pueden ser visibles para usuarios con privilegios suficientes; por ello se recomienda ejecutar el visor con una cuenta de servicio dedicada.

## Limitaciones conocidas

- MJPEG se eligió por simplicidad y compatibilidad del navegador. Consume más ancho de banda/CPU que un pipeline WebRTC o HLS y la vista se limita a 10 fps y 960 px de ancho.
- Se permiten hasta 4 espectadores live simultáneos por cámara.
- El descubrimiento depende de multicast UDP/3702; VLAN, firewalls o routers que bloqueen WS-Discovery pueden impedir encontrar una cámara aunque el alta manual siga funcionando.
- Algunos fabricantes anuncian ONVIF Events pero implementan PullPoint de forma incompleta. El visor reintenta la suscripción; la disponibilidad real depende del firmware.
- La grabación usa Matroska (`.mkv`) y copia los codecs del origen. Si una cámara entrega un formato que FFmpeg no puede remuxear a MKV, la grabación fallará y requerirá una política de transcodificación específica.
- No se implementan todavía PTZ, reproducción de grabaciones desde el navegador, analítica propia, nube ni detección por IA.

## Flags

```text
-listen             dirección HTTP; default 127.0.0.1:8080
-data-dir           directorio de configuración; default data
-recordings-dir     directorio de grabaciones; default recordings
-ffmpeg             ruta de ffmpeg; default ffmpeg
-ui-token           token para modo LAN; también ONVIF_VISOR_TOKEN
```

## Validación del proyecto

Antes de una entrega se deben ejecutar:

```bash
gofmt -w cmd internal web
go test ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor
```
