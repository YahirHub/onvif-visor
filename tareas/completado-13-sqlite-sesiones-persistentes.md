# Tarea 13 — SQLite y sesiones persistentes

## Objetivo
Migrar el login a SQLite embebido, conservar sesiones entre reinicios y permitir administrar/revocar sesiones desde la UI.

## Alcance
- SQLite puro Go dentro de `data/`.
- Usuario bootstrap desde flags/env actuales, contraseña con hash bcrypt.
- Cookie HttpOnly persistente con token aleatorio; sólo hash del token en SQLite.
- Sesiones con expiración, IP, User-Agent, creación y última actividad.
- Endpoints para listar sesiones, revocar una y cerrar todas menos la actual.
- Vista `Sesiones` responsiva.
- Pruebas de persistencia, expiración y revocación.

## Resultado
Completado. `data/accounts.db` usa SQLite puro Go (`modernc.org/sqlite`) con WAL, bcrypt para contraseñas y tokens de sesión aleatorios cuyo SHA-256 es lo único persistido. Las sesiones duran 30 días, sobreviven reinicios y registran IP, User-Agent, alta, última actividad y expiración. La UI incorpora `Sesiones`, cierre individual de sesiones remotas y `Cerrar las demás` manteniendo intacta la sesión actual. Hay pruebas de reapertura de SQLite, autenticación, revocación y logout.
