# Fecha
2026-09-12

# Objetivo
Completar la entrega de SQLite/sesiones/Telegram/multivista añadiendo estados skeleton coherentes y preparar el despliegue final del binario nuevo.

# Skeletons añadidos
- Cámaras: grid skeleton durante la carga inicial y shimmer sobre cada snapshot hasta `load` o `error`.
- Grabaciones: skeleton de cámaras, línea de tiempo y tarjetas; miniaturas de clips mantienen shimmer hasta terminar.
- Eventos: tarjetas skeleton sólo cuando la vista aún no tiene contenido; las miniaturas mantienen shimmer durante los reintentos del snapshot asociado.
- Sesiones: tarjetas skeleton en lugar de spinner genérico mientras se consulta SQLite.
- Descubrimiento: dispositivos skeleton mientras WS-Discovery está activo y limpieza correcta al terminar o fallar.
- Integraciones/Telegram: estado `Consultando…` y panel skeleton durante la primera lectura de configuración, sin parpadeos en el polling posterior.
- Todas las animaciones skeleton respetan `prefers-reduced-motion: reduce`.
- Se añadieron estados de error para que ningún skeleton pueda quedar atascado si falla una petición.

# Validación
- `go test ./...`, `go vet ./...`, build estático `CGO_ENABLED=0` y `git diff --check`: OK.
- `app.js` validado con esbuild: OK.
- Auditoría DOM: 133 IDs HTML, 127 referencias JS por ID, 0 faltantes.
- Smoke HTTP del binario nuevo en loopback: `/`, `/camaras`, `/sesiones`, `/integraciones`, `/app.js`, `/styles.css` responden 200 y contienen los nuevos skeletons.
- Smoke autenticado temporal: login 204, `/api/sessions` 200 con sesión actual y `/api/integrations/telegram` 200 sin token configurado.

# Despliegue real pendiente
El proceso productivo que sigue atendiendo `0.0.0.0:8081` es el binario anterior. La base real todavía no tiene `data/accounts.db`. Para el primer arranque del binario nuevo en LAN hace falta volver a proporcionar las credenciales bootstrap existentes una sola vez. Se verificó que el proceso antiguo sí tiene esas credenciales en su entorno o argumentos, pero no se extrajeron ni copiaron por seguridad. El siguiente paso es recibir usuario/contraseña bootstrap del operador, arrancar el binario nuevo una vez, comprobar SQLite y después ya podrá reiniciarse sin volver a proporcionar la contraseña.
