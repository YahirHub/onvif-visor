# Fecha
2026-09-12

# Objetivo
Añadir autenticación persistente con SQLite, administración de sesiones, corregir la relación de aspecto dinámica de multivista, incorporar fullscreen/responsividad y crear un módulo Telegram para alertas y consulta remota de cámaras.

# SQLite y sesiones
- Se añadió `internal/account` con `modernc.org/sqlite v1.46.1`, compatible con Go 1.24 y compilación `CGO_ENABLED=0`.
- Base `data/accounts.db` con WAL, `busy_timeout`, claves foráneas y permisos 0600; el directorio de datos sigue 0700.
- Contraseñas bcrypt. El token de sesión se genera con 32 bytes aleatorios y sólo su SHA-256 se escribe en SQLite.
- Las sesiones expiran a los 30 días, sobreviven reinicios, registran IP, User-Agent, creación y última actividad.
- `-ui-user`/`-ui-password` pasan a ser credenciales bootstrap: crean/actualizan la cuenta. Una vez existe una cuenta, LAN puede arrancar sin volver a pasar esas variables.
- Nuevos endpoints `/api/sessions`, `DELETE /api/sessions/{id}` y `DELETE /api/sessions/others`.
- Nueva vista `/sesiones` para revisar dispositivos y cerrar todas las sesiones excepto la actual.

# Corrección multivista y fullscreen
- El bug 1:1 provenía del perfil ligero que cambiaba el URI al substream de firmware. Algunas cámaras no conservan la misma relación/resolución entre stream principal y substream.
- `quality=low` ahora mantiene exactamente el mismo RTSP y sólo reduce salida a 5 fps/640 px después de decodificar; por tanto conserva la geometría del stream principal.
- Los panes que salen del monitor se eliminan inmediatamente del grid para impedir celdas implícitas durante la transición; los panes nuevos conservan animación de entrada.
- `ResizeObserver` recalcula el stage al cambiar tamaño.
- Fullscreen usa `requestFullscreen()` sobre `monitorView`; si el navegador no lo soporta se activa modo teatro, ocultando el sidebar.
- En móvil se fuerza un contenedor 16:9 con `object-fit: contain`, navegación inferior horizontal para seis módulos, safe-area y reglas específicas para pantallas <=760 y <=420 px.

# Integración Telegram
- Nueva vista `/integraciones` con tarjeta/módulo Telegram.
- `PUT /api/integrations/telegram` valida el token mediante Bot API `getMe`; el token se cifra con la llave maestra AES-GCM de `internal/store` y nunca se devuelve al cliente.
- `DELETE /api/integrations/telegram` elimina token, chats y códigos.
- `POST /api/integrations/telegram/code` crea un código de 4 dígitos bcrypt, de un solo uso y con expiración de 10 minutos; la UI muestra URL del bot, deep link y comandos `/start CODIGO` y `/login CODIGO`.
- Long polling `getUpdates` con timeout; chats no vinculados no pueden solicitar medios.
- Comandos disponibles: `/camaras`, `/foto <camara>`, `/video <camara>`.
- Fallos de código se limitan a 5 intentos por chat cada 10 minutos.
- Alertas por evento configurables como foto o MP4 de 3–10 s. Sólo se genera un vídeo Telegram simultáneo y existe cooldown de 20 s por cámara para evitar tormentas de alertas.
- `internal/media/clip.go` genera MP4 H.264 limitado para Telegram sin modificar las grabaciones originales.

# Pruebas
- Persistencia de sesión verificada reabriendo la base SQLite en pruebas.
- Revocación, logout y cierre de otras sesiones probados.
- Config Telegram, códigos de un solo uso y chats probados contra SQLite.
- Parser de comandos Telegram probado.
- `getMe` probado contra un servidor HTTP mock, incluyendo respuesta de error.
- `go test ./...`, `go vet ./...` y build estático pasan después de añadir SQLite.
- JavaScript validado con esbuild y auditoría DOM sin IDs referenciados faltantes.

# Limitaciones / validación pendiente
- No se usó un token real de Telegram porque no se proporcionó uno. El panel realiza la validación real al guardarlo; hasta entonces sólo se ha probado el cliente contra Bot API mock.
- El smoke RTSP paralelo no logró obtener frames porque las cámaras ya tenían varias conexiones activas entre el visor principal y watchers. El cambio de aspecto no depende de una inferencia del frontend: se eliminó explícitamente la sustitución por `motionStreamURI`, que era la diferencia entre abrir una cámara sola y agregarla dinámicamente a multivista.
