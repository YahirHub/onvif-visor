# Tarea 22 — ONNX portable, zoom de línea de tiempo y almacenamiento total

## Objetivo
Aclarar y reforzar la instalación de ONNX Runtime en Alpine y otras distribuciones Linux, añadir zoom/pan profesional a la línea de tiempo compatible con touchpad y Android, y mostrar cuánto ocupan en total todos los videos guardados.

## Implementado
- `install.sh` sigue soportando `apk`, `apt`, `dnf`, `yum`, `pacman` y `zypper`.
- La localización de ONNX Runtime ahora consulta `ldconfig`, `/usr/lib`, `/usr/local/lib`, `lib64`, `/lib`, `/lib64` y rutas multiarch obtenidas de GCC.
- Alpine/musl usa `apk add --no-cache onnxruntime`; el paquete nativo aporta una `.so` compatible con musl. Si el paquete no está disponible, el script pide habilitar el repositorio `community` correspondiente y se niega a instalar un tarball glibc incompatible.
- En Linux glibc x86_64/aarch64 el instalador intenta primero el gestor nativo y, si no encuentra ONNX Runtime, descarga el release oficial de Microsoft y copia `libonnxruntime.so*` en `/usr/local/lib`.
- El README distingue modelo `.onnx` de runtime `.so` y documenta ambas rutas de instalación.

## Almacenamiento total
- Nuevo `media.RecordingStorageStats()` con `filepath.WalkDir` sobre todo `recordings/`.
- Cuenta únicamente MKV regulares, de forma case-insensitive, incluyendo carpetas de cámaras que ya no estén configuradas.
- Nuevo endpoint autenticado `GET /api/recordings/storage` con `total_bytes` y `file_count`.
- Grabaciones muestra ahora: cantidad de clips de la cámara, espacio de la cámara, total físico guardado y duración.
- Tras borrar desde la UI, el contador global se descuenta inmediatamente; al recargar se vuelve a obtener el dato físico real del servidor.
- Test unitario valida que sume MKV de múltiples carpetas y descarte otros formatos.

## Zoom/pan de línea de tiempo
- Nuevo viewport horizontal y canvas escalable de 1x a 24x (día completo hasta ~1 hora visible).
- Botones `−`, indicador/restablecer de horas visibles y `+`.
- Regla horaria dinámica: 4 h, 2 h, 1 h, 30 min o 15 min por marca según el zoom.
- PC/touchpad: `Ctrl`/`Cmd` + rueda/gesto hace zoom anclado al cursor.
- Teclado: `+`, `-`, `0` y flechas izquierda/derecha.
- Android/táctil: Pointer Events permiten pellizco de dos dedos para zoom y arrastre horizontal de un dedo para pan.
- `touch-action: pan-y` conserva el scroll vertical natural de la página.
- Después de pinch/pan se suprime brevemente el click para evitar abrir un clip accidentalmente.
- Las etiquetas Hora/Grabación/Personas quedan fijas; sólo se desplaza el lienzo temporal.
- Sin dependencias frontend nuevas.

## Validación
- `go test ./...`: OK.
- `go vet ./...`: OK.
- Build `CGO_ENABLED=1`: OK.
- Build de compatibilidad `CGO_ENABLED=0`: OK.
- `sh -n install.sh`: OK.
- JavaScript validado con esbuild: OK.
- Auditoría DOM: 164 IDs HTML, 157 referencias JS exactas, 0 faltantes.
- `git diff --check`: OK.
- `./install.sh` ejecutado realmente: encontró `/usr/lib/libonnxruntime.so.1`, compiló y actualizó el servicio OpenRC sin reinstalar dependencias innecesarias.
- Servicio de producción: `started`; health OK; endpoint de storage responde 401 sin sesión.
- Smoke no destructivo con instancia temporal sobre las grabaciones reales: 17 MKV y 132,210,913 bytes (~126.1 MB) en ese instante.
- El HTML desplegado contiene `timelineViewport` y el JS desplegado contiene los handlers de Pointer Events.
- SHA-256 del binario en `dist/` y `/opt/onvif-visor/` coincide: `cbbcf1cc532108b91041d77c3cf9156396f5e7108f14ad3f5964628ce19b8f2f`.

## Límite de verificación
No hay navegador/CDP en el host. La interacción real de pinch/touchpad no puede automatizarse aquí; quedó validada estructuralmente y requiere prueba manual final en PC/Android.

## Resultado
Funcionalidad completada, desplegada y lista para probar.
