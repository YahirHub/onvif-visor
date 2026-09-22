# Contexto 22 — Minimalismo de paginación, playback por personas, borrado selectivo y renombre

## Objetivo

Aplicar una mejora compacta a Grabaciones y Configuración:
- paginación más minimalista;
- saltos de persona más precisos;
- interacción tolerante cerca de los puntos amarillos;
- borrado seguro de videos sin personas;
- edición del nombre de cada cámara.

## Paginación

Los enlaces:
- "← Anterior"
- "Siguiente →"

se reemplazaron visualmente por:
- <
- >

Se conservan:
- rutas reales por página;
- href navegable;
- aria-label y title;
- estados disabled;
- números y estado "Página X de Y".

Los botones de flecha son cuadrados de 32x32 px.

## Playback y detecciones de persona

Antes:
- los marcadores amarillos se ubicaban en peak_seconds;
- "Ir a persona" abría peak_seconds;
- el punto visible tenía un área de clic de sólo 8x8 px.

Ahora:
- los marcadores se ubican en start_seconds;
- "Ir a persona" abre start_seconds del mejor segmento;
- la pista compacta de Personas también abre desde start_seconds;
- el punto visible sigue siendo pequeño (8 px) para mantener minimalismo;
- cada punto dispone de un área invisible clicable de 28x28 px.

También se agregó snap del scrubber:
- al soltar el control cerca de un marcador de persona;
- se calcula la distancia visual en píxeles;
- si queda a <=14 px del marcador, la posición se ajusta exactamente a start_seconds;
- la URL ?at= se actualiza al offset resultante.

Así se puede pulsar casi sobre el punto sin necesitar precisión de un píxel.

## Borrado "Sin personas"

Se añadió un botón compacto "Sin personas" en el bloque de eliminación.

Respeta el filtro seleccionado:
- día;
- semana;
- mes;
- mes anterior;
- rango personalizado;
- todas las grabaciones.

La API de borrado por lote acepta:
- without_persons=true

Protecciones:
- nunca elimina archivos activos;
- sólo considera archivos con análisis NanoDet completo usando AnalysisModelKey actual;
- conserva videos donde existe al menos un segmento class_name=person;
- conserva videos pendientes/en análisis;
- si la analítica no está disponible, la operación se rechaza.

La respuesta informa:
- deleted_count;
- skipped_active;
- skipped_persons;
- skipped_unanalyzed.

El frontend informa cuántos clips con personas y cuántos pendientes se conservaron.

El borrado continúa usando deleteRecordingArtifacts, por lo que limpia archivo, analítica y referencias de eventos de forma consistente.

## Renombre de cámara

Nueva API:
- PUT /api/cameras/{id}/name
- body: {"name":"..."}

Reglas:
- trim de espacios;
- nombre obligatorio;
- máximo 80 caracteres;
- persiste en cameras.json;
- actualiza UpdatedAt;
- no modifica ID, endpoint, credenciales, streams ni grabaciones.

La pantalla /camaras/{id}/configuracion incluye ahora un campo Nombre.
Al guardar configuración:
- se persiste el nombre junto con el resto de opciones;
- se recarga la lista de cámaras;
- título, tarjetas y demás vistas usan el nombre actualizado.

Los watchers consultan la cámara actual desde store, así que no necesitan reiniciarse sólo por un cambio de nombre.

## Analítica auxiliar

Se añadió RecordingPersonStates(cameraID), que devuelve dos índices:
- grabaciones con análisis completo del modelo actual;
- grabaciones que tienen segmentos de persona.

Esto permite decidir el borrado sin recorrer consultas por archivo y evita interpretar "sin datos" como "sin personas".

## Pruebas agregadas

- SetName persiste al reabrir store.
- API de nombre acepta un nombre válido.
- API de nombre rechaza nombre vacío.
- RecordingPersonStates:
  - reconoce clips completos con persona;
  - reconoce clips completos sin persona;
  - no considera completos los clips pendientes.
- Borrado sin personas:
  - elimina un clip completo sin persona;
  - conserva clip con persona;
  - conserva clip pendiente;
  - devuelve contadores correctos.

## Validación

- gofmt: OK
- go test ./...: OK
- go vet ./...: OK
- esbuild app.js: OK
- git diff --check: OK
- build CGO: OK
- smoke producción:
  - /api/health -> 200
  - ruta paginada Grabaciones -> 200
  - ruta Configuración -> 200
  - /app.js -> 200
  - /styles.css -> 200
  - PUT /api/cameras/cam-prueba/name sin sesión -> 401 esperado
  - assets contienen Sin personas, cameraSettingsName, personSegmentStart, snapPlaybackOffsetToPerson y estilos de hit-area
- servicio OpenRC: started

## Despliegue

Binario:
- /opt/onvif-visor/onvif-visor
- SHA256: c99f6f3199454ec2161310a988ee0727e0e25b785b27138339d1ac44614a17c2

Backup:
- /opt/onvif-visor/backup-20260922-task36/onvif-visor
- SHA256: 6da22205407bdc4eea92ba3b13f027cf5aaa1ba48f852b1b70e82284f1106624

README no se modificó.
