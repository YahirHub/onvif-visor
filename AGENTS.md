# Reglas del proyecto ONVIF Visor

## Objetivo
Construir un visor local de cámaras ONVIF, escrito en Go, con descubrimiento, alta de dispositivos, visualización en vivo y grabación basada en eventos ONVIF.

## Restricciones
- Backend y aplicación web en Go; los assets HTML/CSS/JS se embeben en el binario.
- Preferir `CGO_ENABLED=0` y un binario estático cuando las dependencias lo permitan.
- FFmpeg puede usarse como ejecutable externo exclusivamente para puente RTSP->navegador y grabación.
- No usar Context7. Para decisiones actuales usar documentación oficial, repositorios oficiales y búsquedas web.
- Mantener la metodología Ponytail: mínima complejidad, sin capas vacías ni dependencias innecesarias.
- No guardar contraseñas de cámaras en texto plano. Cifrar secretos persistidos.
- No registrar contraseñas, URIs RTSP con credenciales ni tokens.
- El servicio debe escuchar solo en loopback por defecto. Si se expone a LAN, requerir token de UI.
- Toda entrada de red debe tener timeouts y límites.
- No asumir compatibilidad total con cámaras no conformantes; degradar funcionalidad y reportar errores claros.

## Calidad
- `gofmt`, `go test ./...`, `go vet ./...` deben pasar antes de entregar.
- Añadir pruebas para parsers, persistencia sensible y lógica de eventos/grabación.
- README y `contexto/` deben reflejar el estado real.
- Git en rama `main`, commits en español.

## Arquitectura preferida
- `cmd/onvif-visor`: arranque y flags.
- `internal/onvif`: descubrimiento y operaciones ONVIF.
- `internal/store`: persistencia cifrada de cámaras/configuración.
- `internal/media`: FFmpeg, live MJPEG y grabación.
- `internal/server`: HTTP API, UI embebida y coordinación.
- `web`: archivos estáticos embebidos.

## Límites del MVP
- Descubrimiento WS-Discovery en la red local.
- Alta manual o desde dispositivos descubiertos.
- Obtención de perfiles/URI RTSP mediante ONVIF.
- Live view de baja latencia usando MJPEG generado por FFmpeg.
- Suscripción/polling de eventos ONVIF y grabación por movimiento/eventos compatibles.
- Grabación directa (`-c copy`) cuando el contenedor/codec lo permitan.
- No implementar PTZ, analíticas propias, reconocimiento ni nube en esta fase.
