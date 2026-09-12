# Fecha
2026-09-12

# Objetivo
Añadir analítica local de personas a las grabaciones del visor, incluyendo archivos históricos y grabaciones largas/programadas, con una pista clicable en la línea de tiempo y consumo acotado para el host actual.

# Decisiones tomadas
- Se eligió NanoDet-Plus-m 320 oficial en ONNX por su tamaño y rendimiento CPU.
- Sólo se usa la clase COCO `person`; no se implementó reconocimiento facial ni tracking persistente.
- El muestreo base es 1 FPS para proteger live, grabación y Telegram.
- ONNX Runtime usa 1 thread intra-op y 1 inter-op, y el Manager serializa toda inferencia con un mutex global.
- Existe un solo worker offline. Las cámaras también mantienen un stream de analítica a 1 FPS sobre `subtype=1` cuando el RTSP lo permite; la inferencia sigue serializada.
- FFmpeg sólo decodifica, escala/pad a 320x320 y entrega BGR24; Go prepara el tensor, ejecuta NanoDet, decodifica DFL, aplica NMS y agrega segmentos.
- La persistencia se separó en `data/analytics.db` con SQLite y permisos `0600`.
- Para evitar reprocesamiento, cada grabación cerrada se identifica por cámara+nombre y huella `size_bytes + modified_unix_ns + model`.
- Para grabaciones activas se consulta `media.Status(cameraID)` y se persiste el offset contra el MKV activo en cuanto aparece una persona.
- Los puntos de detección se fusionan en segmentos si la separación no supera 3 segundos.
- El botón Reanalizar no fuerza una cola infinita: marca análisis como `pending`; los scans periódicos rellenan la cola por tandas y el mapa `queued` evita duplicados activos.
- El modelo y la licencia Apache-2.0 se embeben en el binario. En ejecución local se materializan bajo `data/models/`; instalado como servicio quedan en `/opt/onvif-visor/models/`.
- ONNX Runtime no se embebe: se carga como librería compartida del sistema. Se autodetectan rutas estándar y `-onnxruntime`/`ONVIF_ONNXRUNTIME` permiten una ruta explícita.
- El instalador persiste la ruta explícita de ONNX Runtime en `install.json` para que `service-run` la reutilice después de reinicios.
- `make build` pasa a `CGO_ENABLED=1`; `make build-static` conserva un binario estático que degrada la analítica a “no disponible”.

# Arquitectura actual
```text
Grabación activa RTSP -----> internal/media -----> MKV
          |                         |
          | substream 1 FPS         | Status(cameraID)
          v                         v
 internal/analytics/live -----> MKV + offset
          |
          v
      NanoDet ONNX
          |
          v
 data/analytics.db

MKV cerrado -----> scanner -> cola (1 worker) -> FFmpeg 1 FPS -> NanoDet -> segmentos -> analytics.db

Grabaciones UI -----> /api/analytics/status
                 \--> /api/analytics/persons?camera_id=...
                 \--> pista Personas -> click -> playback en peak_seconds
```

# Librerías usadas
- `github.com/yalue/onnxruntime_go v1.36.0` para invocar ONNX Runtime desde Go/CGO.
- ONNX Runtime 1.x como librería compartida del sistema; host validado con paquete `onnxruntime` de DexOS/Alpine.
- `modernc.org/sqlite` ya existente para `analytics.db`.
- FFmpeg ya existente para extracción de frames y medios.
- Modelo oficial `nanodet-plus-m_320.onnx` del repositorio RangiLyu/nanodet v1.0.0-alpha-1, licencia Apache-2.0 conservada.

# Archivos importantes modificados
- `internal/analytics/assets.go`, `assets/nanodet-plus-m_320.onnx`, `assets/NANODET_LICENSE.txt`
- `internal/analytics/detector.go`, `detector_ort.go`, `detector_stub.go`
- `internal/analytics/manager.go`, `store.go`, `segments.go`
- tests de `internal/analytics/`
- `internal/server/analytics.go`, `server.go`, `handlers.go`
- `cmd/onvif-visor/main.go`
- `internal/service/service.go`, `platform_unix.go`
- `Makefile`, `go.mod`, `go.sum`
- `web/static/index.html`, `app.js`, `styles.css`
- `README.md`

# Problemas encontrados
- El host usa musl; el tarball Linux x64 oficial de ONNX Runtime está pensado para glibc. Se descartó empaquetarlo directamente.
- DexOS/Alpine sí ofrece paquete nativo `onnxruntime`, que resuelve correctamente dependencias y ABI musl.
- La primera versión de Reanalizar intentaba forzar el scan completo y podía perder archivos en bibliotecas mayores que la capacidad del canal. Se cambió a estado `pending` + scans periódicos.
- Una ruta `-onnxruntime` funcionaba al ejecutar manualmente pero inicialmente no se persistía para `service-run`; ahora forma parte de metadata de instalación.
- El modelo intacto hacía que `EnsureModel` retornara antes de restaurar una licencia borrada; ahora modelo y licencia se verifican independientemente.

# Soluciones implementadas
- NanoDet real probado con ONNX Runtime sobre frames de cámaras configuradas.
- Inferencia medida en ~42.45 ms/frame promedio sobre 10 ejecuciones en Cocina, con detección real de persona 0.805.
- Backfill automático y análisis de MKV activo implementados y verificados.
- APIs protegidas para estado, detecciones, análisis y reindexado.
- Pista `Personas` integrada al timeline con geometría continua incluso para clips que cruzan medianoche.
- Click de persona abre la grabación en `peak_seconds`.
- UI actualiza analítica cada 4 s sólo dentro de Grabaciones.
- Servicio final reconstruido e instalado por OpenRC en puerto 8081; hash del binario de `/opt` coincide con `dist/onvif-visor`.
- Modelo desplegado con SHA-256 `4f12723cce3d48e47ca92cb925ba74d97a965c069208edca660bbb9f7ce2c610`.
- Comprobación productiva posterior: 83 grabaciones en estado `complete`, 108 segmentos de persona persistidos.

# Pendientes
- El host no tiene navegador gráfico/CDP; queda pendiente únicamente una revisión visual manual en navegador real de la nueva pista Personas y sus estados responsive/hover. La sintaxis JS, IDs DOM, endpoints y navegación HTTP sí fueron validados.
- Si se distribuye el programa a otras distribuciones Linux, documentar el paquete/ruta de ONNX Runtime específico de cada plataforma o empaquetar una estrategia compatible por distro.

# Próximos pasos
- Usar la aplicación normalmente y revisar falsos positivos/negativos antes de cambiar el umbral 0.35 o el muestreo 1 FPS.
- Si el usuario solicita más clases (vehículo, mascota, etc.), reutilizar el mismo pipeline y añadir pistas/filtros sin incrementar la concurrencia de inferencia.
