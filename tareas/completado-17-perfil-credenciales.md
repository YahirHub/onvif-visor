# Tarea 17 — Perfil y cambio de credenciales

## Objetivo
Permitir que el usuario autenticado cambie su nombre de usuario y contraseña desde un apartado Perfil, validando la contraseña actual y conservando la sesión presente.

## Alcance
- Ruta `/perfil` y API autenticada de perfil.
- Mostrar nombre de usuario actual.
- Cambio de usuario y/o contraseña con contraseña actual obligatoria.
- Validaciones de longitud y colisión de username.
- Bcrypt para la contraseña nueva.
- Al cambiar contraseña, revocar las demás sesiones y conservar la actual.
- Estados loading/error/success y responsive.
- Pruebas de credenciales correctas/incorrectas y sesiones.

## Resultado
Completado. Perfil expone `/api/profile` y `/perfil`, exige contraseña actual para cualquier cambio, valida username, rehashea la contraseña nueva con bcrypt y revoca sólo las demás sesiones cuando cambia la clave. La sesión actual continúa activa y refleja el username nuevo mediante el JOIN de validación. La UI incluye skeleton, confirmación de contraseña, mensajes de éxito/error y diseño móvil. Las pruebas de `internal/account` e `internal/server` y `go vet` pasan.
