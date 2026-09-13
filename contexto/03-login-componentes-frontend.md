# Fecha
2026-09-12

# Objetivo
Corregir el acceso LAN para que el login y todas las interacciones visibles se resuelvan con componentes propios del frontend, sin diálogos HTTP Basic ni mensajes crudos del servidor.

# Decisiones tomadas
- Se eliminó HTTP Basic y la cabecera `WWW-Authenticate`.
- Los assets HTML/CSS/JS quedan accesibles sin sesión para que el navegador siempre pueda renderizar el login.
- Las APIs continúan protegidas cuando se configuran `-ui-user` y `-ui-password`.
- `GET /api/session` informa si el login es requerido y si la sesión actual es válida.
- `POST /api/session` valida usuario/contraseña y crea una cookie aleatoria `HttpOnly`, `SameSite=Strict`, con duración de 12 horas.
- `DELETE /api/session` cierra la sesión eliminando la cookie.
- La cookie permite que el stream MJPEG servido en `<img>` se autentique por mismo origen sin exponer credenciales en la URL.
- `window.prompt` y `window.confirm` fueron eliminados. El login y la confirmación de borrado usan componentes HTML propios.
- Los errores de API se consumen como JSON y se muestran dentro de componentes de la interfaz.

# Arquitectura actual
- Backend Go mantiene la autenticación y la sesión en `internal/server/server.go`.
- Frontend embebido en `web/static/` controla las vistas de arranque, login y dashboard.
- No se agregó ninguna dependencia nueva.

# Librerías usadas
Sin cambios. Se mantiene únicamente la librería estándar de Go para la sesión y el frontend HTML/CSS/JS nativo.

# Archivos importantes modificados
- `internal/server/server.go`
- `internal/server/server_test.go`
- `web/static/index.html`
- `web/static/styles.css`
- `web/static/app.js`
- `README.md`
- `AGENTS.md`
- `contexto/02-implementacion-final.md`
- `tareas/completado-06-login-componentes-frontend.md`

# Problemas encontrados
- El cambio anterior a HTTP Basic provocaba un diálogo nativo del navegador y bloqueaba el render del frontend antes del login.
- `web/static/app.js` todavía conservaba un `window.prompt` del esquema antiguo de token.
- La eliminación de cámaras todavía dependía de `window.confirm`.

# Soluciones implementadas
- Pantalla de login propia con estados de carga, error y sesión cerrada/expirada.
- Sesión por cookie HttpOnly para APIs y MJPEG.
- Modal propio de confirmación para eliminar cámaras.
- Smoke test HTTP: raíz 200 y contiene el formulario de login; ausencia de `WWW-Authenticate`; API 401 sin sesión; login inválido 401; login correcto 204; API 200 con cookie; logout 204; API vuelve a 401.
- Pruebas unitarias de login, protección de rutas, disponibilidad de assets y logout.

# Pendientes
- Validación visual en navegador real de escritorio/móvil cuando haya un navegador disponible en el entorno MCP.
- Prueba extremo a extremo de live MJPEG y eventos contra una cámara ONVIF real de la red local.

# Próximos pasos
- Ejecutar validación final de código y Git.
- Commit del cambio.
- Regenerar el ZIP completo de entrega.
