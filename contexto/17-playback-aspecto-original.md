# Contexto 17 — Playback con aspecto original

## Problema

El reproductor imponía un contenedor 16:9 con límite de altura. Aunque la imagen usaba object-fit: contain, el tamaño del contenedor podía condicionar el cuadro visual y generar una percepción de recorte o márgenes incorrectos.

## Corrección

El reproductor ya no impone aspect-ratio.

El contenedor:
- ocupa el ancho disponible;
- centra con flex;
- no aplica crop;
- no limita la altura del contenedor;
- no oculta overflow del cuadro de video.

La imagen:
- usa width:auto y height:auto;
- max-width:100%;
- max-height limitado al viewport;
- object-fit:contain;
- object-position:center center.

El límite de pantalla se aplica a la imagen, por lo que se reduce proporcionalmente y nunca se recorta.

Las miniaturas de la biblioteca también cambiaron de object-fit:cover a object-fit:contain para mostrar el cuadro completo.

## Verificación con archivo real

Archivo probado:
- resolución original: 2304x1296
- salida del filtro de playback: 1280x720

Ambas relaciones son 16:9 y el filtro FFmpeg mantiene el cuadro completo; no se modificó el pipeline porque no estaba realizando crop.

## Validación

- go test ./...: OK
- go vet ./...: OK
- esbuild app.js: OK
- git diff --check: OK
- salida FFmpeg real verificada: 1280x720 desde 2304x1296
- README sin cambios

## Despliegue

- Binario instalado: /opt/onvif-visor/onvif-visor
- SHA256: 67e3d3e2f1c12a93b73d2ccc339e8c045d4cbee7b7dc3adcf53018b5c7ec6c0b
- Backup: /opt/onvif-visor/backup-20260922-task31/onvif-visor
- SHA256 backup: bff6d924ac241fa223e691370a78b29ab17e8a09e570a8687b24bc92858a996d
- Servicio OpenRC: started
