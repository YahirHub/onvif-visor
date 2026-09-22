# Contexto 21 — Iconos SVG unificados de navegación

## Objetivo

Unificar visualmente los iconos del sidebar de escritorio y la navegación inferior Android/móvil con el mismo estilo vectorial lineal usado en los botones de Ajustes/Eliminar.

## Decisión de arquitectura

Desktop y móvil ya comparten el mismo elemento nav-list. En pantallas <=760 px el sidebar se convierte mediante CSS en una barra inferior horizontal.

Por tanto:
- existe un solo juego de iconos;
- los mismos SVG se muestran en escritorio y Android;
- no se duplica markup ni lógica;
- active/hover y data-view siguen funcionando igual.

## Iconos sustituidos

Se reemplazaron caracteres tipográficos por SVG inline para:
- Cámaras: cámara/lente.
- Multivista: cuadrícula 2x2.
- Grabaciones: reproductor.
- Eventos: actividad/pulso.
- Sesiones: pantalla/dispositivo.
- Perfil: usuario.
- Integraciones: conector.
- Cerrar sesión: salida.

También se actualizó el botón de logout móvil del header para conservar el mismo lenguaje visual.

## Estilo

Los SVG:
- viewBox 24x24;
- fill:none;
- stroke:currentColor;
- stroke-width:1.8;
- extremos y uniones redondeados.

El color continúa heredándose de nav-icon:
- estado normal: gris;
- activo: accent;
- hover: color del nav-item.

Desktop usa SVG de 17x17 px.
Android/móvil usa 18x18 px.

## Tarea pendiente separada

La idea de reconocimiento/agrupación de personas conocidas y mascotas quedó documentada, sin implementación, en:
- tareas/pendientes/reconocimiento-personas-mascotas.md

No se agregó modelo, código ni dependencia relacionada con reconocimiento de identidad.

## Validación

- go test ./...: OK
- go vet ./...: OK
- esbuild app.js: OK
- git diff --check: OK
- build CGO: OK
- producción:
  - /api/health -> 200
  - /camaras -> 200
  - /styles.css -> 200
  - HTML desplegado contiene 8 nav-icon con SVG
  - CSS desplegado conserva navegación Android horizontal por grid-auto-flow:column
- servicio OpenRC: started

## Despliegue

Binario:
- /opt/onvif-visor/onvif-visor
- SHA256: 6da22205407bdc4eea92ba3b13f027cf5aaa1ba48f852b1b70e82284f1106624

Backup:
- /opt/onvif-visor/backup-20260922-task35/onvif-visor
- SHA256: 43dac60dea4b7af61749f2192f25940b7b6ce9f4aca5ae8f6ac709d08949a7b8

README no se modificó.
