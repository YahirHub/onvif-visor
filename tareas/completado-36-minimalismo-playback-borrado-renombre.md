# Tarea 36 — Minimalismo, playback por personas, borrado selectivo y renombre

## Estado
Completada y desplegada.

## Implementado
- Paginación: Anterior/Siguiente reemplazados por < y >.
- Botones de paginación minimalistas 32x32.
- Marcadores amarillos colocados en start_seconds.
- Ir a persona abre desde el inicio real del segmento.
- Timeline de Personas abre también desde start_seconds.
- Área clicable invisible de 28x28 px alrededor de cada punto amarillo.
- Snap del scrubber al marcador más cercano dentro de 14 px visuales.
- Nueva opción "Sin personas" para borrar clips sin detección humana.
- El borrado respeta el filtro temporal seleccionado.
- Sólo elimina clips ya analizados completamente con el modelo actual.
- Conserva activos, clips con personas y clips pendientes de análisis.
- Nuevo campo Nombre en Configuración de cámara.
- Nueva API PUT /api/cameras/{id}/name.
- Nombre persistente, obligatorio y máximo 80 caracteres.

## Validación
- gofmt: OK.
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Build CGO: OK.
- Smoke producción: OK.
- Servicio OpenRC: started.
- README sin cambios.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: c99f6f3199454ec2161310a988ee0727e0e25b785b27138339d1ac44614a17c2
- Backup: /opt/onvif-visor/backup-20260922-task36/onvif-visor
- SHA256 backup: 6da22205407bdc4eea92ba3b13f027cf5aaa1ba48f852b1b70e82284f1106624
