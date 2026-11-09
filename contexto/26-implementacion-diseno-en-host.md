# Fecha
2026-11-09

# Objetivo
Implementar las correcciones de diseño de DESIGN.md y desplegarlas en el host, actualizando la instalación existente de ONVIF Visor y reiniciando su servicio OpenRC.

# Decisiones tomadas
- Compilar en el host (Alpine, musl, CGO activado) y no en el sandbox (Debian, glibc): un binario del sandbox no arrancaría en el host.
- Alcance de esta entrega: la capa de corrección visual completa de DESIGN.md. La división de app.js en módulos, la navegación de cuatro secciones y la consolidación de breakpoints quedan para la fase de frontend por módulos, ya registrada como deuda visible.
- Sustituir Lucide por un sprite SVG local de 25 símbolos y un icons.js propio que imita createIcons, porque app.js ya llamaba a window.lucide de forma protegida y así no hay que tocar su lógica.
- Conservar copia del binario anterior en /opt/onvif-visor/onvif-visor.bak-20261109-diseno para rollback inmediato.

# Arquitectura actual
- Sin cambios de arquitectura. Cambios solo en web: styles.css, index.html y el nuevo icons.js.

# Librerías usadas
- Se elimina la dependencia externa de Lucide por CDN (unpkg). Ninguna librería nueva.

# Archivos importantes modificados
- web/static/styles.css: tokens de DESIGN.md (--text-2 #96a5b5 sustituye a --muted-2 en texto, --text-3 solo decorativo, --video-bg), fuera Inter de la pila de fuentes, suelo tipográfico de .75rem en todo el CSS, radios de insignias de 999px a 6px y de 14/10/9/5/4px a 12/8/8/6/6px, fuera el degradado radial del login, eyebrows ocultos y objetivos táctiles de 40px (44px en móvil).
- web/static/index.html: fuera el script de unpkg, sprite SVG local con 25 símbolos y carga de /icons.js antes de app.js.
- web/static/icons.js (nuevo): createIcons local sin dependencias.
- En el host: fuente de compilación en /root/onvif-visor-build y binario actualizado en /opt/onvif-visor/onvif-visor.

# Problemas encontrados
- El sandbox no tenía FFmpeg y el servidor no arranca sin él en media.New: se instaló ffmpeg solo en el sandbox para validar. Con eso, la suite completa pasa.
- La compilación para el host debe hacerse en el propio host por musl y CGO.

# Soluciones implementadas
- Validación en sandbox: gofmt limpio, go vet correcto, go test ./... completo en verde y prueba de humo con health, index con 25 símbolos, sin unpkg e icons.js y styles.css servidos con 200.
- Despliegue: parada con rc-service, sustitución del binario (md5 idéntico al compilado), arranque con rc-service. Health correcto, 7 procesos FFmpeg de cámaras y analítica de nuevo en marcha, grabación reanudada y la compresión interrumpida por el reinicio retomada sola.

# Pendientes
- deuda-tecnica: app.js sigue siendo un monolito de 3.834 líneas; partir en módulos ES por vista en la fase 4.
- deuda-tecnica: la navegación sigue con siete secciones; mover sesiones, perfil e integraciones a Ajustes en la fase 4.
- deuda-tecnica: breakpoints sin consolidar a 1100/760/480; se conservan los actuales para no alterar layouts en producción sin revisión pantalla a pantalla.
- El usuario debe revisar visualmente la interfaz en su navegador y confirmar que iconos y tamaños se ven bien con sus cámaras.

# Próximos pasos
1. Revisión visual del usuario en http://dex:8081 o la dirección LAN que use.
2. Fase 4 completa: frontend por módulos y navegación de cuatro secciones.
3. Hub de streams compartido (tarea 3 de Alpine) cuando el usuario lo priorice.
