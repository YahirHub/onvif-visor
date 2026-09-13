# Tarea 23 — README y preparación documental para GitHub

## Objetivo
Dejar la documentación principal lista para publicar el repositorio en GitHub sin cambiar el comportamiento del proyecto.

## Implementado
- Se mantuvo el `README.md` existente y se amplió para GitHub.
- Añadido estado del proyecto y sección de inicio rápido.
- Añadido ejemplo LAN usando variables de entorno para el usuario bootstrap.
- Añadida matriz de compatibilidad de `install.sh`: Alpine, Debian/Ubuntu, Fedora/RHEL, Arch, openSUSE, systemd, OpenRC, runit y SysV.
- Documentada la diferencia entre compatibilidad Linux general y los fallbacks automáticos amd64/arm64.
- Añadida sección específica sobre publicación del repositorio y datos privados.
- Documentada la licencia de terceros de NanoDet y la ausencia deliberada de una licencia raíz propia del proyecto.
- `.gitignore` reforzado para `.env`, `.env.*`, temporales y `coverage.out`, conservando `!.env.example`.
- Se reemplazaron dos referencias a una IP LAN real en documentación histórica de `contexto/` por descripciones genéricas antes de preparar el commit público.
- Creado `contexto/12-documentacion-publicacion-github.md`.

## Validación
- README revisado contra el estado real de instalación y arquitectura.
- `.gitignore` conserva fuera del repositorio `data/`, `recordings/`, `dist/` y secretos habituales.
- No se creó ni inventó una licencia legal para el código del proyecto.
- `git diff --check`: OK; revisión del contenido staged completada antes del commit.

## Resultado
Documentación pública preparada para GitHub. Queda como decisión del propietario elegir una licencia raíz si desea conceder permisos explícitos de redistribución/contribución.
