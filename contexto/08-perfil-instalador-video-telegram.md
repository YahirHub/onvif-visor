# Fecha
2026-09-12

# Objetivo
Agregar cambio de credenciales desde Perfil, un instalador de servicio inteligente para gestores Linux comunes y corregir/expandir los clips solicitados por Telegram.

# Perfil
- Nueva ruta `/perfil` y API `GET/PUT /api/profile`.
- El usuario autenticado puede cambiar username y/o contraseña.
- La contraseña actual es obligatoria para confirmar la operación.
- Username: 3–64 caracteres, letras Unicode, números, punto, guion y guion bajo.
- Contraseña nueva: opcional, mínimo 8 y máximo 128 caracteres.
- El cambio usa bcrypt y una transacción SQLite.
- Al cambiar contraseña se revocan todas las demás sesiones del usuario; la sesión actual se conserva y refleja el username nuevo.
- La UI incluye skeleton, confirmación de contraseña, estados loading/error/success y layout móvil.

# Instalador de servicio
- Nuevo paquete `internal/service`, adaptado conceptualmente del instalador de Lilith MCP.
- `--install` detecta systemd, OpenRC, runit o SysV init, en ese orden y validando que el manager esté realmente utilizable.
- Elevación automática con sudo, doas o pkexec cuando el proceso no es root.
- Instala el binario atómicamente en `/opt/onvif-visor/onvif-visor` y crea `/usr/local/bin/onvif-visor` como symlink.
- La primera instalación migra `data/` y `recordings/` desde `--source-dir` sólo si el destino está vacío. Upgrades posteriores preservan los datos instalados.
- Se rechazan symlinks dentro del árbol migrado y los archivos sensibles migrados quedan 0600, directorios 0700.
- `install.json` sólo almacena schema, manager, listen, ruta de ffmpeg y timestamp; no contiene credenciales.
- El servicio ejecuta `service-run`, que lee la metadata y usa `/opt/onvif-visor/data` y `/opt/onvif-visor/recordings`.
- `--service status|start|stop|restart` abstrae el manager real.
- `--remove` deshabilita/remueve el servicio y binario, pero conserva data y recordings.
- Antes de iniciar una instalación LAN se comprueba que haya una cuenta SQLite; los flags bootstrap pueden crearla sin guardar la contraseña en metadata.

# Vídeo Telegram
- `video_seconds` admite 3–60 segundos. El panel define la duración por defecto para alertas y `/video <camara>`.
- `/video <camara> <segundos>` permite override, por ejemplo `/video Cocina 30`.
- El comando intenta primero resolver el argumento completo como nombre de cámara; así nombres como `Patio 2` no se interpretan automáticamente como duración.
- Los clips ya no usan `nobuffer`/`low_delay` porque esos flags perjudicaban la estabilidad de un archivo compartible.
- FFmpeg usa timestamps de reloj, `+genpts+discardcorrupt`, warm-up de 500 ms, `setpts=PTS-STARTPTS`, 12 fps, máximo 1280 px, H.264 CRF 23, maxrate 1.8 Mbps, GOP de 2 s, 2 threads y faststart.
- El timeout escala con la duración solicitada y el long polling del bot no se bloquea mientras se genera un clip largo.
- Sólo se permite una captura Telegram simultánea para no saturar CPU/cámara; si ya hay otra, el bot responde que lo intente después.

# Verificación real
- Filtro FFmpeg sintético: 1280x720, 12 fps y 2.000 s exactos.
- Cámara real `Cocina`, clip solicitado de 6 s: H.264, 1280x720, 12 fps, duración 6.000 s, 1,251,308 bytes.
- Cámara real `Cocina`, clip solicitado de 30 s: H.264, 1280x720, 12 fps, duración 30.000 s, 6,901,003 bytes.
- `go test` y `go vet` focales de account/media/server/service/CLI pasan durante la implementación.

# Despliegue real del instalador
- Se detuvo limpiamente el proceso manual que atendía `0.0.0.0:8081` antes de migrar, evitando copiar SQLite/grabaciones abiertas.
- `./dist/onvif-visor --install --source-dir .` detectó OpenRC y terminó correctamente en el host de validación.
- Servicio activo: `/opt/onvif-visor/onvif-visor service-run`, habilitado en el runlevel `default`.
- Symlink CLI: `/usr/local/bin/onvif-visor -> /opt/onvif-visor/onvif-visor`.
- Metadata instalada: manager `openrc`, listen `0.0.0.0:8081`, ffmpeg `ffmpeg`; no contiene secretos.
- Permisos comprobados: `data/` y `recordings/` 0700; `accounts.db` y `master.key` 0600.
- Migración de grabaciones comprobada por tamaño: 321.1 MiB tanto en origen como en `/opt/onvif-visor/recordings`.
- Smoke instalado: `/camaras`, `/multivista`, `/grabaciones`, `/eventos`, `/sesiones`, `/perfil` e `/integraciones` responden 200; `/api/health` 200 y `/api/profile`/`/api/sessions` responden 401 sin sesión.
- `onvif-visor --service status` reporta el servicio iniciado.
