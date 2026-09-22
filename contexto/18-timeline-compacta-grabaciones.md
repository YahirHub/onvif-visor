# Contexto 18 — Timeline compacta en Grabaciones

## Objetivo

La línea de tiempo inteligente estaba consumiendo demasiado espacio vertical y desplazaba la cuadrícula de videos, que debe ser el contenido principal de la biblioteca.

## Nueva jerarquía

La vista de Grabaciones queda organizada así:
1. Resumen del día.
2. Línea de tiempo compacta.
3. Archivos de video y paginación.
4. Controles de eliminación.

Los controles de eliminación se movieron debajo de los archivos/paginación para que una acción administrativa no interrumpa el contenido principal.

## Timeline compacta

Por defecto se muestran solo:
- Grabación.
- Personas.

La fila de Personas utiliza marcadores amarillos para diferenciarla visualmente de las grabaciones.

El encabezado, regla horaria, pistas y controles se redujeron para ocupar menos altura. La ayuda de gestos queda oculta en modo compacto.

Se agregó el botón:
- "Ver todas las clases"

Al pulsarlo:
- se expande la vista multiclase existente;
- aparecen todas las clases detectadas;
- se conservan zoom, fecha, reanálisis y saltos a video;
- el botón cambia a "Mostrar solo personas".

La vista compacta es la predeterminada al cargar la aplicación.

## Texto contextual

Debajo de la timeline compacta se muestra un resumen como:
- "533 detecciones de persona · pulsa un marcador para ir directamente al video."

Cuando el resumen diario todavía no está disponible, se usa como fallback el número de tramos de persona cargados en la timeline.

En modo expandido se informa cuántas clases están presentes.

## Resumen diario

Se redujeron padding, gaps y tamaño visual de los indicadores para conservar la información sin quitar protagonismo a los archivos.

## Archivos

La sección "Archivos" aparece inmediatamente después de la timeline. No se modificaron:
- tarjetas;
- paginación real por URL;
- acciones Abrir archivo / Ir a persona;
- reproducción dedicada;
- detecciones amarillas dentro del playback.

## Validación

- go test ./...: OK
- go vet ./...: OK
- esbuild web/static/app.js: OK
- git diff --check: OK
- smoke de producción:
  - /api/health -> 200
  - ruta paginada de Grabaciones -> 200
  - /app.js -> 200
  - /styles.css -> 200
  - assets contienen timelineDetailsBtn, timelineExpanded y estilos compactos
- servicio OpenRC: started

## Despliegue

Binario instalado:
- /opt/onvif-visor/onvif-visor
- SHA256: 1afb590d13a6ca5a62aa9c187b2d8b55096fcfd880112e5d50f95173f7ae8c74

Backup previo:
- /opt/onvif-visor/backup-20260922-task32/onvif-visor
- SHA256: 67e3d3e2f1c12a93b73d2ccc339e8c045d4cbee7b7dc3adcf53018b5c7ec6c0b

README no se modificó.
