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

# Despliegue real
Completado manualmente después de recibir las credenciales bootstrap del operador. Se creó `data/accounts.db` con permisos 0600, se validó el login SQLite, se detuvo el proceso anterior y se arrancó el binario nuevo en `0.0.0.0:8081`. `/camaras`, `/multivista`, `/grabaciones`, `/eventos`, `/sesiones` e `/integraciones` respondieron 200 y `/api/sessions` respondió 401 sin sesión, confirmando que la protección persistente quedó activa. El repositorio permaneció limpio durante esa migración operativa.
