# DESIGN.md — ONVIF Visor

Dirección visual y de interacción del visor. Este documento manda sobre cualquier
decisión de interfaz: si una pantalla no puede explicar cómo cumple estas reglas,
no está terminada. Se sigue junto a Ponytail (simplicidad) y Antislop (nada de
plantilla genérica). Cada decisión importante lleva su motivo en una línea, en el
registro del final.

## 1. Qué es este producto, visualmente

ONVIF Visor es una consola de videovigilancia local. No es un dashboard SaaS, no
es una landing y no es un panel de administración genérico.

- El vídeo es el contenido. Todo lo demás (navegación, botones, textos) existe
  para llegar al vídeo, entender su estado y actuar sobre él en segundos.
- Se usa en tres contextos reales: una pantalla de monitoreo que se mira de
  pasada, un ordenador donde se revisan grabaciones y un móvil donde se comprueba
  una alerta. Las tres deben funcionar sin aprendizaje previo.
- La interfaz debe parecer un instrumento: precisa, sobria, legible a un metro
  de distancia en su información clave (qué cámara, si graba, si hay señal).

## 2. Diales Antislop

| Dial | Valor | Motivo |
| --- | --- | --- |
| ENERGY | 1, calmado | Una consola de seguridad no saluda ni impresiona: informa. |
| RHYTHM | 1, uniforme | La uniformidad aquí es deliberada: cada cámara y cada grabación se leen con el mismo patrón, sin sorpresas. |
| MOTION | 1, mínimo | El único movimiento que importa ya está en pantalla y es el vídeo. |

## 3. Tema y paleta

Tema oscuro único, con motivo: el visor se usa con poca luz, el fondo oscuro no
compite con la imagen de las cámaras y el negro del vídeo se integra con la
pantalla en vez de recortarse contra un fondo claro. No hay tema claro: mantener
dos temas bien hechos costaría el doble en cada pantalla, y el motivo del oscuro
es funcional, no decorativo.

Máximo: neutros + 1 acento + colores de estado con significado fijo.

| Token | Valor | Uso |
| --- | --- | --- |
| --bg | #080b0f | Fondo de la aplicación |
| --rail | #0c1015 | Barra de navegación |
| --surface | #10151b | Paneles y superficies |
| --surface-2 | #151b22 | Superficie elevada, controles |
| --surface-3 | #1a222b | Hover y selección suave |
| --video-bg | #040608 | Fondo del escenario de vídeo |
| --text | #f4f7fa | Texto principal |
| --text-2 | #96a5b5 | Texto secundario, también en tamaño pequeño |
| --text-3 | #647180 | Solo decoración, bordes y texto grande, nunca texto pequeño |
| --line | rgba(148,163,184,.12) | Separadores |
| --accent | #50d0ad | Acción principal, foco y selección |
| --danger | #ff7379 | Error, peligro y estado grabando |
| --on-dark | #06130f | Texto sobre el acento |

Reglas de color:

- El acento no decora. Si algo es verde azulado, es accionable, está
  seleccionado o tiene el foco. Nada más usa ese color.
- Grabando se indica con punto en --danger más la palabra GRABANDO. En vivo se
  indica con la palabra EN VIVO. El estado nunca se comunica solo con color.
- Sin degradados, sin brillos, sin fondos con rejilla y sin cristal esmerilado.
  La única excepción heredada, el halo del login, desaparece en el rediseño.
- Las clases de detección de la analítica (persona, vehículo, animal) se
  distinguen en la línea de tiempo con forma y etiqueta, no solo con color.

Contrastes medidos sobre esta paleta (WCAG, cálculo de luminancia):

| Combinación | Ratio | Uso permitido |
| --- | --- | --- |
| --text sobre --bg | 18,34 | Todo |
| --text-2 sobre --bg | 7,84 | Todo, incluido texto pequeño |
| --text-2 sobre --surface | 7,29 | Todo, incluido texto pequeño |
| --accent como texto sobre --bg | 10,30 | Texto y foco |
| --on-dark sobre --accent | 9,90 | Botón principal |
| --danger sobre --bg | 7,49 | Texto de error y estado |
| --text-3 sobre --bg | 3,96 | Prohibido en texto normal, solo grande o decorativo |

El antiguo --muted-2 (#647180) se usaba en texto pequeño, por ejemplo la regla
de la línea de tiempo a 0,52 rem con #586676. Queda corregido por este documento:
ningún texto baja de --text-2 ni de 12 px.

## 4. Tipografía

- Una sola familia: la del sistema (system-ui y equivalentes). Motivo: el visor
  es local y debe pintar al instante sin descargar fuentes; Inter estaba
  declarado en el CSS pero nunca se cargaba, así que la identidad real ya era
  la del sistema.
- Monoespaciada del sistema (ui-monospace) solo para datos que se comparan o se
  copian: horas, duraciones, direcciones IP, nombres de archivo y tokens.
- Escala cerrada, sin tamaños sueltos: 12 (metadatos), 13 (secundario), 14
  (cuerpo), 16 (título de sección), 20 (título de pantalla), 28 (solo login).
- Prohibido texto por debajo de 12 px en cualquier vista, también en la línea
  de tiempo y en las marcas sobre el vídeo. Si no cabe, se abrevia el contenido,
  no se encoge la letra.

## 5. Espacio, radios y bordes

- Espaciado en múltiplos de 4: 4, 8, 12, 16, 24, 32. El aire separa bloques;
  no se usan tarjetas dentro de tarjetas para crear jerarquía.
- Tres radios y nada más: 6 para vídeo y elementos pequeños, 8 para controles y
  paneles, 12 para diálogos y la tarjeta de login. Círculo completo solo para
  puntos de estado y el avatar. El CSS actual usa diez valores de radio
  distintos: quedan reducidos a estos.
- Bordes de 1 px con --line. Sombras solo en diálogos y menús flotantes, una
  sola definición (--shadow). Las superficies planas no llevan sombra.

## 6. Estructura y navegación

Cuatro secciones. Sesiones, perfil e integraciones dejan de ser secciones y
pasan dentro de Ajustes, porque se usan una vez y ocupan navegación todos los días.

1. **En directo.** La pantalla de inicio y la principal. Rejilla de cámaras en
   vivo; un clic abre la cámara a pantalla completa dentro de la vista.
2. **Grabaciones.** Lista por cámara y día con la línea de tiempo como
   herramienta central, no como adorno.
3. **Eventos.** Lista cronológica que salta al segundo exacto de la grabación.
4. **Ajustes.** Cámaras, sistema, integraciones, sesiones y perfil, en pestañas.

Envoltura:

- Escritorio: raíl lateral estrecho (icono y etiqueta, 184 px; 68 px solo
  iconos por debajo de 1100 px) y cabecera fina con el título de la sección y
  una sola acción principal.
- Móvil (760 px o menos): barra inferior con las cuatro secciones, objetivos
  táctiles de al menos 44 px, y el vídeo siempre por encima de los controles.
- Un solo punto focal por pantalla: en En directo y en cámara, el vídeo; en
  Grabaciones, la línea de tiempo; en Eventos, la lista. Todo lo demás calla.
- El escenario de vídeo usa --video-bg, separación de 2 px entre cámaras y
  conserva siempre la relación de aspecto original de cada cámara, sin recortes.
  Las rejillas de multivista existentes (1, 2, 3, 4, 6 y 9) se conservan.

Cabecera de sección sin eyebrow. El patrón actual de etiqueta en mayúsculas
sobre cada título (MONITOREO, WS-DISCOVERY, CONFIRMACIÓN) desaparece: el título
dice qué es la pantalla y no necesita un rótulo encima.

## 7. Componentes

- **Botón principal.** Uno por vista como máximo, en --accent. Su texto es la
  acción concreta: Detectar cámaras, Agregar cámara, Guardar cambios. Prohibidos
  los textos vacíos tipo Empezar, Explorar o Saber más.
- **Botones secundarios y fantasma.** Para el resto de acciones, en ese orden de
  importancia. Lo destructivo va en --danger y siempre pide confirmación en un
  diálogo que nombra el objeto: «Eliminar la cámara Patio» y no «¿Estás seguro?».
- **Objetivos táctiles.** Botones de icono de 40 px en escritorio y 44 px en
  táctil. Los actuales de 32 px quedan por debajo del mínimo.
- **Chips de estado.** Rectángulo pequeño con punto y palabra: EN VIVO,
  GRABANDO, SIN SEÑAL, DESCONECTADA, ANALIZANDO. Sin parpadeos ni pulsos: el
  estado es un dato, no una animación.
- **Tarjetas de cámara.** La miniatura manda: imagen 16/9, nombre y estado
  debajo o en una banda inferior legible. Las acciones (ajustes, eliminar)
  aparecen al pasar el ratón en escritorio y están siempre visibles en táctil.
- **Línea de tiempo.** Regla horaria legible (12 px, --text-2), tramos de
  grabación en --surface-3, eventos con marca, forma y etiqueta por clase,
  zoom con botones de 40 px y desplazamiento por teclado.
- **Diálogos.** El elemento dialog nativo, radio 12, un título, el contenido y
  dos acciones como máximo. Se cierran con Escape y devuelven el foco a quien
  los abrió.
- **Iconos.** Un sprite SVG local, en línea, trazo 1,8 y currentColor. Sin CDN,
  sin fuentes de iconos, sin Unicode ni emojis haciendo de icono. La lista es la
  que usa la interfaz actual: cámara, rejilla, vídeo, actividad, ajustes,
  usuario, integración, salir, buscar, actualizar, más, cerrar, volver, pantalla
  completa, menos y sliders. Si un icono no está en el sprite, se dibuja o no se
  pone: nunca se improvisa con un carácter.

## 8. Movimiento

- Transiciones de 120 a 160 ms solo en hover, selección y aparición de diálogos.
- El indicador de carga es el spinner actual, solo mientras se carga de verdad.
- Nada de pulsos infinitos, contadores animados ni entradas escalonadas.
- Se respeta prefers-reduced-motion, que el CSS ya contempla: con esa
  preferencia, ni siquiera quedan las transiciones.

## 9. Texto e idioma

- Todo en español, como el resto del proyecto. Frases cortas que nombran cosas
  reales: cámaras, grabaciones, eventos, almacenamiento.
- Prohibidas las palabras vacías: inteligente como reclamo, potente, avanzado,
  sin fisuras, de última generación. La analítica se describe por lo que hace:
  «Detecta personas, vehículos y animales en esta cámara».
- Errores en dos partes: qué pasó y qué se puede hacer. «Sin señal. La cámara
  no responde; comprueba que está encendida y en la misma red.» Nunca códigos
  crudos del servidor ni trazas.
- Estados vacíos con la acción siguiente, no con ilustraciones: «No hay
  cámaras» y los botones Detectar y Agregar cámara.

## 10. Accesibilidad, bloqueos de entrega

Ninguna pantalla se entrega sin cumplir:

- Contraste según la tabla de la sección 3, verificado al cambiar cualquier
  token de color o de texto.
- Foco visible en todo elemento interactivo: contorno de 2 px en --accent con
  desplazamiento de 2 px, como ya hace el CSS actual.
- Todo se puede hacer con teclado, en orden lógico, incluidos la línea de
  tiempo, la multivista y los diálogos.
- Zoom del navegador al 200 % sin perder acciones ni texto.
- Objetivos táctiles de 44 px en móvil y texto que nunca baja de 12 px.
- El estado nunca se comunica solo con color, ni en chips, ni en eventos, ni
  en la línea de tiempo.

## 11. Lo que no se hace aquí

- Plantilla de dashboard con sidebar, tarjetas de estadísticas y gráfica sin
  pregunta que responder.
- Rejillas bento, tarjetas de funciones copiadas, insignias en forma de cápsula
  y puntos decorativos de estado.
- Degradados azul-morado, cristal esmerilado, brillos y fondos con rejilla.
- Iconos de librería cargados desde un CDN o sustituidos por caracteres Unicode.
- Reescribir esta dirección para una sola pantalla porque «queda mejor». Si una
  regla estorba de verdad, se cambia aquí, con su motivo, y se aplica a todas.

## 12. Registro de decisiones

| Decisión | Motivo |
| --- | --- |
| Tema oscuro único | Se usa con poca luz y el fondo no compite con el vídeo. |
| Acento único #50d0ad, heredado | Ya es la identidad del producto y su contraste está medido. |
| --text-2 sustituye a --muted-2 en texto pequeño | #647180 da 3,96 sobre el fondo y no llega al mínimo de 4,5. |
| Tipografía del sistema, sin fuentes descargadas | El visor es local: pinta al instante y no depende de Internet. |
| Monoespaciada solo para datos | Horas, IPs y archivos se comparan carácter a carácter. |
| Nada por debajo de 12 px | La regla de tiempo actual a 0,52 rem no se puede leer. |
| Tres radios (6, 8, 12) | El CSS actual usa diez valores sin sistema. |
| Cuatro secciones en vez de siete | Sesiones, perfil e integraciones son ajustes, no tareas diarias. |
| Sin eyebrows sobre los títulos | El título ya dice qué es la pantalla; el rótulo es plantilla. |
| Un botón principal por vista | Dos acciones principales compiten y ninguna manda. |
| Estado con palabra, no solo color y sin parpadeo | El estado debe leerse de pasada y también con daltonismo. |
| Iconos en sprite SVG local | El CDN quedaba bloqueado por la propia CSP y el visor debe funcionar sin Internet. |
| Vídeo con su aspecto original y fondo #040608 | Recortar una cámara de seguridad es perder información, no diseño. |
| Diales 1, 1, 1 | Es un instrumento de monitoreo, no una pieza de exhibición. |
| Errores con causa y salida | Un mensaje crudo deja al usuario sin siguiente paso. |

## 13. Cómo se aplica en la fase de frontend

1. Sustituir los tokens de :root por los de este documento y corregir los usos
   de --muted-2 en texto pequeño.
2. Reducir radios, tamaños de letra y breakpoints a los sistemas cerrados de
   aquí. Breakpoints oficiales: 1100, 760 y 480. Los demás se eliminan.
3. Cambiar la navegación a cuatro secciones y mover sesiones, perfil e
   integraciones a Ajustes.
4. Sustituir Lucide por el sprite local y retirar el script de unpkg.
5. Revisar pantalla a pantalla contra las secciones 7, 9 y 10 antes de darla
   por terminada, y anotar en el contexto del proyecto cualquier desviación y
   su motivo.
