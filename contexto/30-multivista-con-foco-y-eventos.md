# 30 · Multivista con foco, preferencias y eventos sin ola eterna

Peticiones del usuario (2026-11-09) sobre la versión de `6e9fdbd`.

## Multivista

- La selección de cámaras se guarda en `localStorage`
  (`onvif.monitor.selected`) desde `renderMonitor`, el punto por el que
  pasan todos los cambios: sobrevive a recargas y cierres del navegador.
- Clic en un cuadro lo pone en foco: ocupa todo el escenario con una
  animación breve de zoom (`paneFocusIn`). Se sale con la tecla Esc o con
  el botón flotante «Ver todas» (pensado para Android, visible en todas
  las plataformas). Aviso «Pulsa Esc…» solo en escritorio.
- Opción «Segundo plano al ampliar» (rueda de opciones junto a Cámaras,
  persistida en `onvif.monitor.keepBackground`): sin marcar, al entrar en
  foco las demás cámaras se pausan (se les quita el `src`), ahorrando
  datos y batería; marcándola, siguen en directo aunque no se vean y al
  salir con Esc la rejilla está viva al instante, sin reconectar.
- El foco no cambia la calidad de ningún stream: es solo disposición, así
  entrar y salir no reconecta la cámara enfocada.
- Al salir de la vista de monitor el foco se olvida.

## Eventos: la «ola oscura» en el teléfono

La ola es el brillo de carga (`skeletonSweep`) de las miniaturas de
evento. No se quitaba porque la extracción del fotograma en el servidor
tarda, el cliente reintentaba 3 veces con esperas (~6 s por miniatura) y
el sondeo de eventos cada 2.5 s refrescaba también con la vista oculta.

- La ola caduca a los 8 s (`setTimeout` en `loadEventThumbnail`); la
  imagen aparece cuando llega, ya sin animación.
- Reintentos de miniatura: de 3 a 1.
- El sondeo de eventos solo refresca cuando la vista de Eventos está en
  pantalla.
