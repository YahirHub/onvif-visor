# Fecha
2026-09-12

# Objetivo
Transformar la interfaz del ONVIF Visor en un centro de monitoreo tipo VMS/NVR profesional: galería de cámaras por miniaturas, visor individual, multivista adaptable y biblioteca de grabaciones con línea de tiempo.

# Decisiones tomadas
- Se investigaron interfaces de referencia actuales de Synology Surveillance Station, UniFi Protect y Milestone XProtect antes del rediseño.
- Se tomó como referencia adicional el panel web seguro en Go disponible en la biblioteca del usuario para conservar un shell de panel autocontenido, responsive, con assets embebidos y modales propios.
- No se copió la arquitectura multiusuario/SQLite del panel base porque el ONVIF Visor actual sigue siendo una aplicación local/single-operator; agregarla ahora sería complejidad no requerida.
- La pantalla principal post-login muestra snapshots JPEG bajo demanda, no streams continuos, para evitar abrir un proceso FFmpeg live por cada tarjeta.
- Click en una cámara abre el stream live MJPEG existente.
- La multivista admite hasta 9 cámaras. Los layouts se adaptan: 1, 2 mitad/mitad, 3 con cámara principal + dos secundarias, 4 en 2x2, 5-6 en 3x2 y 7-9 en 3x3. En móvil se apilan verticalmente.
- La selección de multivista puede modificarse mientras se observa el monitor.
- La biblioteca de grabaciones indexa únicamente MKV dentro del directorio seguro de cada cámara.
- La API de grabaciones nunca devuelve rutas físicas del host; usa únicamente camera_id y nombre de archivo validado.
- Los nombres de grabación deben ser basenames `.mkv`; se rechaza traversal.
- La línea de tiempo es de 24 horas y posiciona cada clip según hora local de inicio y duración aproximada.
- La duración del clip se calcula a partir del timestamp inicial codificado en el nombre y la última modificación del archivo. Se eligió para evitar agregar ffprobe como segunda dependencia de runtime.
- La reproducción web usa FFmpeg para convertir un MKV almacenado a MJPEG a velocidad real, con seek por segundos, máximo 4 reproducciones concurrentes y sin audio.
- El MKV original se puede descargar de forma autenticada.
- La sesión sigue siendo la sesión frontend HttpOnly/SameSite=Strict implementada anteriormente.

# Arquitectura actual
- `internal/media/snapshot.go`: captura JPEG para thumbnails de cámaras.
- `internal/media/library.go`: catálogo seguro de grabaciones, thumbnails, playback MJPEG y límites de concurrencia.
- `internal/server/handlers.go`: endpoints de snapshots, biblioteca, reproducción y descarga.
- `web/static/index.html`: shell VMS, navegación, galería, monitor, multivista, almacenamiento/timeline y eventos.
- `web/static/styles.css`: sistema visual dark responsive y layouts adaptables.
- `web/static/app.js`: navegación y estados frontend, selección multivista, biblioteca, timeline y controles de reproducción.

# Librerías usadas
Sin dependencias nuevas. Se mantiene Go standard library, `github.com/0x524a/onvif-go v1.1.4` y FFmpeg como único ejecutable externo de medios.

# Archivos importantes modificados
- `internal/media/manager.go`
- `internal/media/snapshot.go`
- `internal/media/library.go`
- `internal/media/library_test.go`
- `internal/server/server.go`
- `internal/server/handlers.go`
- `web/static/index.html`
- `web/static/styles.css`
- `web/static/app.js`
- `README.md`
- `tareas/completado-07-galeria-visor-multivista.md`
- `tareas/completado-08-grabaciones-timeline.md`
- `tareas/en-proceso-09-integracion-vms-entrega.md`

# Problemas encontrados
- El dashboard original abría live directamente dentro de cada card, lo que no escalaba bien para muchas cámaras.
- No existía endpoint de snapshot ni vista previa ligera.
- `/api/recordings` solo reportaba grabadores activos; no indexaba archivos históricos.
- Los MKV almacenados no tenían UI de consulta/reproducción.
- El entorno de desarrollo actual no incluye Chromium/Firefox/Node/Bun/Deno; no es posible realizar una auditoría visual automatizada en navegador desde este host.
- No hay todavía credenciales reales de la cámara física detectada ni clips reales generados por ella para una prueba extremo a extremo.

# Soluciones implementadas
- Galería de thumbnails reales con búsqueda y actualización manual.
- Estados de thumbnail `EN LÍNEA` / `SIN VISTA` en función de la captura real.
- Monitor individual y multivista hasta 9 cámaras.
- Selector lateral de cámaras que permite agregar/quitar streams mientras se monitorea.
- Biblioteca por cámara con totales de clips, bytes y tiempo aproximado.
- Filtro por fecha y timeline de 24 horas.
- Miniaturas de grabaciones generadas con FFmpeg.
- Reproductor MJPEG con pausa/reanudación y seek.
- Descarga del MKV original.
- Pruebas unitarias para traversal y parsing/listado de archivos.
- Smoke test local: health, cámaras y `/api/recordings` respondieron 200; con almacenamiento vacío `/api/recordings` devolvió `[]`.

# Pendientes
- Validación visual manual/automática en un navegador real.
- Prueba live real y multivista contra una cámara ONVIF de la red local cuando se proporcionen sus credenciales.
- Generar al menos una grabación real y validar thumbnails/playback/timeline extremo a extremo.
- PTZ sigue fuera de alcance.

# Próximos pasos
- Completar documentación y validación final.
- Reiniciar el servicio LAN con el build final.
- Commit del rediseño.
- Regenerar ZIP de entrega sin `data/` ni `recordings/` y validar el archivo.
