# Fecha
2026-09-13

# Objetivo
Dejar la documentación principal del proyecto preparada para publicar el repositorio en GitHub sin cambiar el comportamiento del visor.

# Decisiones tomadas
- Se conserva el `README.md` existente porque ya documentaba el proyecto ampliamente; se mejora en lugar de crear un segundo README.
- La portada del README ahora incluye estado del proyecto e instalación rápida para que un usuario nuevo pueda entender y levantar el visor sin leer toda la documentación primero.
- Se documenta explícitamente que `install.sh` está soportado en Linux y qué familias de gestores/init systems intenta automatizar.
- Se aclara la compatibilidad amd64/arm64 para descargas automáticas y que otras arquitecturas dependen de paquetes nativos.
- Se documenta qué datos nunca deben subirse a un repositorio público y se refuerza `.gitignore` para archivos `.env`, temporales y cobertura.
- No se crea una licencia raíz automáticamente porque elegir MIT, Apache-2.0, GPL u otra licencia cambia los términos legales del proyecto y requiere decisión del propietario.
- El README deja claro que `internal/analytics/assets/NANODET_LICENSE.txt` corresponde al componente NanoDet y no constituye una licencia general del proyecto.

# Arquitectura actual
No cambia la arquitectura de ejecución. La estructura publicada sigue siendo:

```text
cmd/onvif-visor/      ejecutable principal
internal/             backend, ONVIF, media, analítica, cuentas y Telegram
web/                  interfaz HTML/CSS/JS embebida
contexto/             decisiones y continuidad técnica
tareas/               historial de tareas Ponytail
install.sh             build + dependencias + instalación del servicio
README.md              documentación pública principal
```

# Librerías usadas
No se agregaron ni actualizaron dependencias.

# Archivos importantes modificados
- `README.md`
- `.gitignore`
- `contexto/12-documentacion-publicacion-github.md`
- `tareas/completado-23-readme-publicacion-github.md`

# Problemas encontrados
- El README ya existía y era técnico/completo, pero no tenía una entrada corta para usuarios que lleguen desde GitHub.
- La matriz real de compatibilidad de `install.sh` estaba repartida entre varias secciones.
- El repositorio no tiene un archivo `LICENSE` raíz para el código propio; sólo existe el aviso/licencia de NanoDet.
- `.gitignore` protegía datos, grabaciones y binarios, pero no archivos `.env` ni algunos artefactos de desarrollo comunes.

# Soluciones implementadas
- Añadida sección **Inicio rápido** con instalación local y ejemplo LAN seguro mediante variables de entorno.
- Añadido bloque **Estado del proyecto** con alcance local/LAN y dependencia de firmware/FFmpeg/ONNX Runtime.
- Añadida tabla **Compatibilidad de instalación** para Alpine, Debian/Ubuntu, Fedora/RHEL, Arch, openSUSE e init systems.
- Añadida sección **Publicación del repositorio y datos privados** con rutas que no deben subirse.
- Añadida sección **Licencia del proyecto** que informa que todavía no existe una licencia raíz y evita atribuir al código completo la licencia del modelo NanoDet.
- `.gitignore` ahora ignora `.env`, `.env.*` (excepto `.env.example`), `*.tmp` y `coverage.out` además de los datos/artefactos ya protegidos.
- Se sanitizaron referencias a una IP LAN real en `contexto/03-login-componentes-frontend.md` y `contexto/04-redisenio-vms-multivista-grabaciones.md`, además de una ruta absoluta del workspace en `contexto/08-perfil-instalador-video-telegram.md`; la documentación actual ya no publica esos detalles operativos.

# Pendientes
- Decidir y agregar una licencia raíz (`LICENSE`) antes de definir formalmente permisos de redistribución/contribución pública.
- Los commits históricos anteriores a esta limpieza pueden seguir conteniendo la antigua IP LAN. Si se quiere que el historial público tampoco la revele, habría que reescribir el historial local antes del primer push; no se hizo automáticamente porque esa operación cambia SHAs existentes.
- Opcional: agregar capturas de pantalla o GIF del panel cuando se quiera mejorar la presentación visual del repositorio.
- Opcional: añadir `CONTRIBUTING.md` y `SECURITY.md` si el repositorio se abre a contribuciones externas.

# Próximos pasos
- Revisar una última vez `git status` antes del push.
- Crear el repositorio/remoto de GitHub y hacer push de `main` cuando el propietario lo decida.
- Elegir licencia del proyecto si se desea que terceros reutilicen o contribuyan código bajo términos explícitos.
