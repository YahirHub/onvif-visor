# Fecha
2026-09-13

# Objetivo
Reforzar la instalación portable de ONNX Runtime en Linux, añadir zoom/pan profesional a la línea de tiempo de Grabaciones y mostrar el espacio total real ocupado por todos los videos almacenados.

# Decisiones tomadas
- El modelo NanoDet `.onnx` y ONNX Runtime son piezas distintas: el primero contiene grafo/pesos y `libonnxruntime.so` ejecuta la inferencia nativa desde Go.
- En Alpine/musl sólo se usa un paquete ONNX Runtime compilado para musl mediante `apk`; no se instala el tarball Linux oficial de Microsoft porque está orientado a glibc.
- En Linux glibc se prefiere una instalación existente o el paquete nativo; si el gestor no ofrece ONNX Runtime, `install.sh` descarga el release oficial de Microsoft para x64/aarch64.
- La búsqueda de la `.so` ya no asume `/usr/lib`: usa `ldconfig`, `lib`/`lib64`, `/usr/local` y directorios multiarch detectados con GCC.
- El total de almacenamiento debe representar los archivos físicos realmente guardados, no sólo las cámaras actualmente configuradas. Por eso el backend recorre todo `recordings/` y suma archivos MKV regulares, incluyendo carpetas de cámaras eliminadas.
- El zoom de timeline es puramente frontend: la geometría sigue normalizada a las 24 horas, y el lienzo se ensancha hasta 24x dentro de un viewport horizontal. No se duplican datos ni consultas.
- Zoom máximo 24x, equivalente a aproximadamente 1 hora visible; zoom mínimo 1x, día completo.
- Los gestos se implementan con APIs nativas del navegador: Wheel Events para touchpad/ratón y Pointer Events para pinch/drag táctil, sin dependencia JS nueva.

# Arquitectura actual
```text
Modelo NanoDet .onnx
       │
       ▼
Go + onnxruntime_go ── carga ──> libonnxruntime.so
                                      │
                Alpine/musl: apk onnxruntime
                glibc: paquete nativo o release oficial Microsoft

/api/recordings/storage
       │
       └─ filepath.WalkDir(recordings/) -> suma *.mkv -> total_bytes/file_count

Timeline 24h normalizada
       │
       ├─ zoom 1x..24x -> ancho canvas 100%..2400%
       ├─ Ctrl/Cmd + rueda/pinch touchpad -> zoom anclado al cursor
       ├─ Android 2 dedos -> pinch zoom
       └─ Android 1 dedo horizontal -> pan; vertical sigue desplazando la página
```

# Librerías usadas
- Go standard library (`filepath.WalkDir`, `os`, HTTP existente).
- APIs nativas del navegador (`wheel`, Pointer Events, scroll horizontal).
- FFmpeg/ffprobe existentes.
- `github.com/yalue/onnxruntime_go` y ONNX Runtime ya existentes; no se agregó dependencia.

# Archivos importantes modificados
- `install.sh`
- `internal/media/library.go`
- `internal/media/library_test.go`
- `internal/server/handlers.go`
- `internal/server/server.go`
- `web/static/index.html`
- `web/static/app.js`
- `web/static/styles.css`
- `README.md`
- `tareas/completado-22-onnx-portable-zoom-timeline-almacenamiento.md`

# Problemas encontrados
- La detección anterior de ONNX Runtime sólo inspeccionaba `/usr/lib` y `/usr/local/lib`, insuficiente para distribuciones que usan `lib64` o rutas multiarch.
- Un tarball oficial Linux de Microsoft no debe tratarse como reemplazo universal en Alpine, porque Alpine usa musl y necesita un binario compatible con musl.
- La línea de tiempo fija de 24 horas hacía difíciles de seleccionar clips o detecciones muy cortos.
- El frontend mostraba espacio por cámara, pero no existía una cifra que reflejara todos los MKV físicamente almacenados.

# Soluciones implementadas
- `find_onnxruntime` consulta primero `ldconfig`, después rutas `lib`/`lib64`, `/usr/local` y multiarch.
- El error musl indica explícitamente habilitar el repositorio `community` apropiado en Alpine y volver a ejecutar el script; evita instalar un binario glibc incompatible.
- Nuevo `RecordingStorageStats()` recorre todo el root de grabaciones, cuenta MKV y suma bytes.
- Nuevo endpoint protegido `GET /api/recordings/storage` devuelve `total_bytes` y `file_count`.
- Grabaciones muestra cuatro indicadores: clips de cámara, espacio de cámara, total guardado global y duración de cámara.
- El total global se actualiza localmente tras borrados hechos desde la UI y se recalcula físicamente al volver a cargar/refrescar Grabaciones.
- Nuevo viewport/canvas de timeline con regla dinámica. La densidad de marcas cambia entre 4 h, 2 h, 1 h, 30 min y 15 min según zoom.
- Botones `−`, indicador de horas visibles y `+`; el indicador también restablece a 24 h.
- Touchpad/PC: `Ctrl`/`Cmd` + rueda/gesto hace zoom alrededor del cursor; teclado `+`, `-`, `0` y flechas también funciona.
- Android/touch: pinch de dos dedos hace zoom y arrastre horizontal de un dedo recorre el día; el desplazamiento vertical de la página permanece disponible.
- Se suprimen clicks inmediatamente después de un gesto para evitar abrir una grabación por accidente.

# Validación
- `go test ./...`: OK.
- `go vet ./...`: OK.
- Build principal `CGO_ENABLED=1`: OK.
- Build de compatibilidad `CGO_ENABLED=0`: OK.
- `sh -n install.sh`: OK.
- JavaScript validado con esbuild: OK.
- Auditoría DOM previa a la entrega: 164 IDs HTML, 157 referencias JS exactas, 0 faltantes.
- `git diff --check`: OK.
- `./install.sh` ejecutado realmente en el servidor: detectó ONNX Runtime en `/usr/lib/libonnxruntime.so.1`, compiló, actualizó el servicio y OpenRC quedó `started`.
- Smoke HTTP de almacenamiento sobre el directorio real mediante instancia temporal sin autenticación: `132210913` bytes y `17` MKV en ese instante (~126.1 MB). La instancia temporal se detuvo y sus datos se limpiaron.
- Producción: `/api/health` OK; `/api/recordings/storage` devuelve 401 sin sesión; `/grabaciones` y `/app.js` sirven los nuevos elementos/lógica de timeline.
- Binario desplegado y `dist/onvif-visor` coinciden por SHA-256: `cbbcf1cc532108b91041d77c3cf9156396f5e7108f14ad3f5964628ce19b8f2f` antes del commit final (el contenido del binario ya es el final de esta tarea).

# Pendientes
- No hay pendiente funcional obligatorio.
- El host no dispone de navegador/CDP, así que el gesto real de touchpad y pinch Android requiere comprobación visual/manual en un dispositivo aunque sintaxis, DOM y estructura responsive estén validados.

# Próximos pasos
- Probar Ctrl/pinch de touchpad en PC y pellizco/arrastre en Android sobre `/grabaciones`.
- Ajustar el límite de 1 hora visible sólo si en uso real se necesita llegar a minutos/segundos con mayor zoom.
- Si el almacenamiento crece mucho, considerar política automática de retención/cuota como tarea separada.
