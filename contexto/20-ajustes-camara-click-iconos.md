# Contexto 20 — Botón Ajustes y centrado de acciones

## Problema

El botón Ajustes de las tarjetas cambiaba la URL pero no asignaba el ID de cámara a state.cameraSettingsID antes de llamar navigate(). loadCameraSettingsView() encontraba el ID vacío y regresaba inmediatamente a Cámaras, por lo que visualmente parecía que el clic no hacía nada.

Los iconos ⚙ y × dependían de las métricas tipográficas de la fuente, por lo que no se veían perfectamente centrados aunque el botón usara place-items:center.

## Corrección

Se agregó openCameraSettings(cameraID):
- valida que la cámara exista;
- asigna state.cameraSettingsID;
- navega a /camaras/{id}/configuracion.

El botón Ajustes llama ahora a este helper y ejecuta preventDefault + stopPropagation.

El botón Eliminar también detiene propagación para evitar cualquier interacción accidental con la miniatura.

La capa camera-hover-actions ahora tiene z-index propio y pointer-events activos.

Los caracteres tipográficos de Ajustes/Eliminar fueron reemplazados por SVG inline:
- Ajustes: icono de controles/sliders.
- Eliminar: cruz vectorial.

Los botones tienen tamaño fijo 30x30, padding 0 y los SVG se centran mediante grid, eliminando diferencias de baseline entre fuentes/sistemas.

## Validación

- go test ./...: OK
- go vet ./...: OK
- esbuild app.js: OK
- git diff --check: OK
- sin IDs HTML duplicados detectados
- build CGO: OK
- smoke producción:
  - /api/health -> 200
  - /camaras/cam-prueba/configuracion -> 200
  - /app.js -> 200
  - /styles.css -> 200
  - app desplegada contiene openCameraSettings, asignación de cameraSettingsID y stopPropagation
  - CSS desplegado contiene z-index e iconos SVG centrados
- servicio OpenRC: started

## Despliegue

Binario:
- /opt/onvif-visor/onvif-visor
- SHA256: 43dac60dea4b7af61749f2192f25940b7b6ce9f4aca5ae8f6ac709d08949a7b8

Backup:
- /opt/onvif-visor/backup-20260922-task34/onvif-visor
- SHA256: d974e6d5c66ddc9a2c1c862acf1ddb3e13c785730873e192c4f2626bea455044

README no se modificó.
