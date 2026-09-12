# Fecha
2026-09-12

# Estado
MVP ONVIF Visor terminado y validado localmente.

# Objetivo cumplido
Aplicación monobinaria Go con interfaz web embebida para descubrir cámaras ONVIF, agregarlas, consultar perfiles/RTSP, visualizar vídeo en vivo y grabar a partir de eventos ONVIF PullPoint.

# Decisiones finales
- Go mínimo declarado: 1.24.0.
- Dependencia ONVIF: `github.com/0x524a/onvif-go v1.1.4`.
- FFmpeg instalado en el host y usado únicamente para RTSP->MJPEG y remux de grabaciones.
- Build del visor con `CGO_ENABLED=0`.
- Live view: MJPEG, 10 fps, ancho máximo 960 px y máximo 4 viewers por cámara.
- Grabaciones: MKV con `-c copy`; un evento activo extiende el post-roll de una grabación existente.
- Descubrimiento por WS-Discovery en interfaces activas con multicast, máximo 16.
- Credenciales persistidas cifradas con AES-256-GCM; RTSP persistido sin userinfo.
- Loopback por defecto. Cualquier bind no-loopback exige token.

# Implementación
- `cmd/onvif-visor/main.go`: flags, bind seguro, shutdown.
- `internal/onvif/`: discovery, perfiles, URLs y Events/PullPoint.
- `internal/store/`: estado JSON y cifrado de contraseñas.
- `internal/media/`: FFmpeg live y grabación.
- `internal/server/`: API, UI, autenticación y watchers de eventos.
- `web/`: UI estática embebida.

# Pruebas realizadas
- `gofmt` correcto.
- `go test ./...` correcto.
- `go vet ./...` correcto.
- Validación Lilith/code_validate completa correcta.
- Opciones FFmpeg `mpjpeg` y `boundary_tag` verificadas en el FFmpeg instalado.
- Smoke test: `/api/health` 200 y UI embebida servida correctamente.
- Seguridad LAN: bind `0.0.0.0` sin token rechazado; con autenticación, API devuelve 401 antes de sesión y 200 después de token válido.
- Build `CGO_ENABLED=0`; `ldd` informa `Not a valid dynamic program`.

# Limitaciones verificadas/documentadas
- No se dispuso de una cámara física ONVIF en este entorno para una prueba extremo a extremo contra firmware real.
- La compatibilidad final de PullPoint y de los tópicos de movimiento depende del fabricante/firmware.
- MJPEG prioriza compatibilidad y simplicidad; no es tan eficiente como WebRTC/HLS.
- FFmpeg sigue siendo una dependencia externa de runtime permitida por el requisito original.

# Pendientes futuros opcionales
- PTZ.
- Reproductor/listado de grabaciones desde la UI.
- WebRTC para live de menor ancho de banda.
- Mapeo configurable de tópicos ONVIF por fabricante.
- Métricas y retención automática de grabaciones.
