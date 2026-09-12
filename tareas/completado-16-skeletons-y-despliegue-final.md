# Tarea 16 — Skeletons y despliegue final

## Objetivo
Completar la entrega pendiente: estados skeleton en cargas relevantes, validación final, reinicio del servicio real en :8081 con el binario nuevo y commit Git.

## Alcance
- Skeletons para cámaras, eventos, sesiones, grabaciones, descubrimiento e Integraciones/Telegram sin parpadeo durante polling.
- Skeleton de miniaturas hasta que llegue snapshot/thumbnail.
- Respetar prefers-reduced-motion.
- Revalidar JS, DOM, Go test/vet/build y diff check.
- Migrar el servicio real a SQLite conservando las credenciales bootstrap actuales sin exponer secretos.
- Smoke HTTP del servicio en :8081 y comprobación de rutas nuevas.
- Commit final en español.

## Avance
- Skeletons implementados en todas las cargas relevantes y miniaturas asíncronas.
- Estados de error evitan skeletons atascados.
- `prefers-reduced-motion` aplicado.
- Go test/vet/build, esbuild, DOM y smoke HTTP/autenticado pasan.
- El binario nuevo está construido en `dist/onvif-visor`.

## Pendiente para cerrar
- Reiniciar `0.0.0.0:8081` con el binario nuevo.
- Crear `data/accounts.db` usando las credenciales bootstrap actuales una sola vez.
- Smoke del servicio real y marcar tarea completada.
- Commit final tras confirmar el despliegue, o commit de código previo si se necesita esperar las credenciales.

Las credenciales del proceso anterior no se extraen ni se copian automáticamente por seguridad; deben volver a proporcionarse explícitamente para la migración inicial.

## Resultado final
Completado. La migración real se hizo manualmente con el usuario bootstrap suministrado por el operador, se creó `data/accounts.db` con permisos 0600, se detuvo el proceso anterior y se levantó el binario nuevo en `0.0.0.0:8081`. Las rutas públicas de la SPA respondieron 200 y los endpoints protegidos devolvieron 401 sin sesión, confirmando que la autenticación SQLite quedó activa.
