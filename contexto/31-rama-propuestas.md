# 31 · Rama `propuestas`: interfaz simplificada, Android y funciones ligeras

Rama desechable creada el 2026-11-09 a petición del usuario para probar
todo junto sin tocar producción (main sigue en `40dc2bb`). Si no convence,
se borra la rama y no queda nada. Contenido, un commit por bloque:

- **Grabaciones simplificada** (`8c45d77`): las cifras del día (archivos,
  tamaño, hora pico, entradas, cruces, clases) viven en un «Resumen del
  día» plegado y cerrado por defecto (su estado es preferencia). La línea
  de tiempo conserva fecha y zoom y pierde botones secundarios y textos
  de gestos. Tarjetas con solo franja horaria, duración y tamaño; el
  listado se titula «Hoy», «Ayer» o la fecha.
- **Pasada Android** (`153a11e`): la barra inferior ahora se desplaza en
  horizontal — con 7 secciones en una rejilla de 4 columnas y 58 px, tres
  quedaban fuera de pantalla —, campos a 16 px (sin zoom automático al
  escribir), objetivos táctiles de 40–48 px, diálogos como hojas
  inferiores a ancho completo y safe-area en barra y selector.
- **PWA y atajos** (`c40f1fc`): manifiesto + iconos + service worker de
  red primero (la API y los directos nunca se cachean) para instalar el
  visor en Android. Atajos de escritorio: 1–9 abre o enfoca cámaras y F
  alterna pantalla completa en multivista.
- **Tramo y protección** (`567c6db`): «Marcar inicio/fin» en el
  reproductor descarga solo ese tramo en MP4 (remux con `-ss`/`-t`; con
  copia de stream el corte cae en el fotograma clave anterior). Cada
  grabación se puede proteger desde su tarjeta: el conjunto vive en
  `protected_recordings.json` junto a los datos y bloquea el borrado
  individual (409), el por lote (las salta y las cuenta) y la limpieza
  automática de retención.
- **Aviso sin señal** (`cefda34`): si la miniatura en vivo de una cámara
  no carga, la portada muestra una franja con su nombre hasta que vuelva.

La instancia de prueba de esta rama corre en el host en el puerto 18099
con una copia de los datos; producción (8081) no se ha tocado.
