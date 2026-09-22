# Tarea 35 — Iconos SVG unificados de navegación

## Estado
Completada y desplegada.

## Implementado
- Sidebar de escritorio convertido de símbolos tipográficos a SVG lineales.
- Navegación inferior Android/móvil reutiliza exactamente los mismos SVG.
- Iconos nuevos para Cámaras, Multivista, Grabaciones, Eventos, Sesiones, Perfil e Integraciones.
- Cerrar sesión del sidebar y header móvil también actualizado a SVG.
- Estados active/hover conservados mediante currentColor.
- Responsive conserva navegación horizontal móvil existente.
- Idea futura de reconocimiento de personas/mascotas archivada en tareas/pendientes sin implementar.

## Validación
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Build CGO: OK.
- Smoke de producción: OK.
- Servicio OpenRC: started.
- README sin cambios.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: 6da22205407bdc4eea92ba3b13f027cf5aaa1ba48f852b1b70e82284f1106624
- Backup: /opt/onvif-visor/backup-20260922-task35/onvif-visor
- SHA256 backup: 43dac60dea4b7af61749f2192f25940b7b6ce9f4aca5ae8f6ac709d08949a7b8
