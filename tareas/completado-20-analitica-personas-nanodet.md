# Tarea 20 — Analítica de personas con NanoDet

## Objetivo
Indexar grabaciones existentes, nuevas y programadas para detectar presencia de personas y mostrar intervalos clicables en la línea de tiempo sin bloquear live ni grabación.

## Diseño implementado
- NanoDet-Plus-m 320 oficial en ONNX, usando únicamente la clase COCO `person`.
- ONNX Runtime CPU como librería compartida externa; ruta autodetectada o configurable/persistible con `-onnxruntime`.
- Modelo oficial embebido en el binario y materializado junto con su licencia Apache-2.0.
- Un solo worker offline y una sola inferencia simultánea global, con muestreo base de 1 FPS.
- ONNX Runtime limitado a 1 thread intra-op + 1 inter-op; FFmpeg usa 1 thread para decodificación del indexador.
- `analytics.db` separado de `accounts.db`, modo `0600`.
- Backfill automático de grabaciones cerradas no indexadas mediante huella tamaño+mtime+modelo.
- Grabaciones activas/programadas analizadas sobre substream ligero a 1 FPS; una detección se vincula al MKV activo y a su offset sin esperar al cierre.
- Segmentos de persona fusionados con tolerancia de 3 s para evitar marcadores fragmentados.
- Timeline de Grabaciones con pista `Personas`; click abre el clip en el offset de mayor confianza.
- API autenticada de estado, segmentos, progreso por grabación y reanálisis.
- Botón `Reanalizar`: marca análisis terminados como pendientes y el scanner los vuelve a encolar por tandas, sin duplicar trabajos ya activos.
- Si ONNX Runtime/modelo no están disponibles, el visor continúa funcionando y la analítica reporta estado no disponible.
- Build estático `CGO_ENABLED=0` conservado como variante degradada sin inferencia NanoDet.

## Validación realizada
- `gofmt -w cmd internal` OK.
- `go test ./...` OK.
- `go vet ./...` OK.
- Build principal `CGO_ENABLED=1` OK.
- Build estático de compatibilidad `CGO_ENABLED=0` OK.
- `git diff --check` OK.
- JavaScript validado con esbuild OK.
- Auditoría DOM: 148 IDs HTML, 141 referencias JS exactas, 0 faltantes.
- Inferencia real sobre la cámara Cocina: persona detectada con confianza 0.805; promedio de 10 inferencias ~42.45 ms/frame en este host.
- Backfill real encontró intervalos de personas en grabaciones existentes.
- Smoke autenticado del endpoint `POST /api/analytics/reindex`: HTTP 202 y cola continuó procesando sin duplicar el trabajo activo.
- Servicio final desplegado con OpenRC en `0.0.0.0:8081`.
- SHA-256 del binario instalado coincide con `dist/onvif-visor`.
- Modelo desplegado SHA-256: `4f12723cce3d48e47ca92cb925ba74d97a965c069208edca660bbb9f7ce2c610`.
- `install.json` conserva la ruta personalizada de ONNX Runtime usada por `service-run`.
- En la comprobación productiva posterior al backfill: 83 grabaciones completas indexadas y 108 segmentos de persona almacenados.

## Limitaciones conscientes
- 1 FPS prioriza bajo consumo; apariciones de menos de un segundo pueden quedar entre muestras.
- Sólo se detecta `person`; no hay reconocimiento facial, identidad, tracking persistente ni nube.
- ONNX Runtime es una dependencia del sistema. En el DexOS/Alpine validado se instaló con `apk add --no-cache onnxruntime`.
- El host no dispone de navegador gráfico/CDP, por lo que no se hizo validación visual pixel a pixel; se validaron sintaxis JS, IDs DOM, rutas HTTP y APIs reales.

## Resultado
Funcionalidad implementada, probada con cámaras/grabaciones reales y desplegada en el servicio instalado.
