# Tarea 06 — Login y feedback completamente frontend

## Objetivo
Sustituir la autenticación HTTP Basic y cualquier diálogo nativo del navegador por componentes propios de la interfaz embebida.

## Alcance
- Login visual dentro de ONVIF Visor con usuario y contraseña.
- Sesión mediante cookie HttpOnly para que el stream MJPEG en `<img>` funcione sin credenciales en la URL.
- `/api/session` para consultar estado, iniciar sesión y cerrar sesión.
- Rutas estáticas siempre visibles; APIs protegidas salvo health y sesión.
- Estados de sesión expirada, login inválido, loading y error mostrados en componentes frontend.
- Reemplazar `window.prompt` y `window.confirm` por UI propia.
- Mantener JSON de API como contrato interno, sin mostrar respuestas crudas del servidor al usuario.
- Actualizar documentación/contexto y validar build/pruebas.

## Criterios de aceptación
- Abrir la URL sin sesión muestra el formulario de login, no un popup HTTP Basic ni texto del servidor.
- Credenciales inválidas se muestran dentro del formulario.
- Credenciales válidas abren el dashboard y permiten cargar APIs y MJPEG mediante cookie.
- Sesión expirada devuelve al login visual.
- Eliminar cámara usa modal propio.
- No quedan usos de `window.prompt`, `window.confirm` ni `WWW-Authenticate`.
- `gofmt`, `go test ./...`, `go vet ./...` y build estático pasan.

## Resultado
Completado y validado. El visor ahora muestra un login propio del frontend, mantiene sesión mediante cookie HttpOnly/SameSite=Strict, muestra sesión expirada y errores dentro de la interfaz, y usa un modal propio para confirmar el borrado de cámaras. Se eliminaron HTTP Basic, `WWW-Authenticate`, `window.prompt` y `window.confirm` del código ejecutable. Pasaron `gofmt`, `go test ./...`, `go vet ./...`, build estático y smoke tests HTTP del flujo login/sesión/logout.

La validación visual con navegador real no pudo ejecutarse porque el cliente MCP disponible no expone herramienta de navegador; se verificó la entrega de HTML/CSS/JS y el flujo HTTP por CLI.
