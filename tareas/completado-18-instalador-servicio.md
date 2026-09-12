# Tarea 18 — Instalador inteligente de servicio

## Objetivo
Agregar `--install` inspirado en Lilith MCP para instalar ONVIF Visor en `/opt/onvif-visor` y administrarlo con el gestor de servicios disponible.

## Alcance
- Detectar systemd, OpenRC, runit y SysV init.
- Elevación con sudo/doas/pkexec cuando haga falta.
- Copia atómica del binario a `/opt/onvif-visor/onvif-visor`.
- Preservar/migrar `data/` y `recordings/` en primer install y no sobrescribir datos en upgrades.
- Symlink `/usr/local/bin/onvif-visor`.
- `service-run`, `--service status|start|stop|restart` y `--remove` preservando datos.
- Configuración persistente de listen/ffmpeg en metadata de instalación.
- Pruebas de detección/render de gestores y documentación.

## Resultado
Implementado. `--install` detecta systemd, OpenRC, runit o SysV; eleva con sudo/doas/pkexec si hace falta; instala el binario atómicamente en `/opt/onvif-visor`, crea el symlink `/usr/local/bin/onvif-visor`, migra `data/` y `recordings/` sólo si el destino está vacío y nunca los sobrescribe en upgrades. `install.json` conserva listen/ffmpeg sin secretos y `service-run` arranca desde esa metadata. También existen `--service status|start|stop|restart` y `--remove`, que preserva datos y grabaciones. Antes de iniciar el servicio, la instalación comprueba que una escucha LAN tenga al menos una cuenta SQLite; opcionalmente puede crearla con los flags bootstrap. Hay pruebas para scripts de los cuatro gestores, quoting y migración segura sin symlinks. `go test`/`go vet` del instalador y CLI pasan.

## Despliegue real
Validado en este host OpenRC. El instalador migró el servicio a `/opt/onvif-visor`, habilitó `onvif-visor` en el runlevel `default`, creó el CLI en `/usr/local/bin`, preservó 321.1 MiB de grabaciones y dejó `accounts.db`/`master.key` con 0600. El proceso activo ejecuta `service-run` desde `/opt` y los smokes HTTP/protección pasan.
