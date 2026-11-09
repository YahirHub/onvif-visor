# Fecha
2026-11-09

# Objetivo
Corregir el hueco superior y el error de CSP que introdujo el despliegue del diseño (contexto 26).

# Decisiones tomadas
- El sprite de iconos deja de ocultarse con el atributo style y pasa a la clase CSS .icon-sprite (posición absoluta, tamaño 0), servida desde styles.css, que la CSP sí permite.

# Arquitectura actual
- Sin cambios. Dos archivos: web/static/index.html y web/static/styles.css.

# Librerías usadas
- Ninguna.

# Archivos importantes modificados
- web/static/index.html: el svg del sprite usa class="icon-sprite" en vez de style="display:none".
- web/static/styles.css: nueva regla .icon-sprite. Desplegado en el host recompilando y reiniciando el servicio OpenRC, con el mismo procedimiento del contexto 26.

# Problemas encontrados
- El sprite se insertó con style="display:none". La CSP del servidor (style-src 'self') bloquea los estilos inline, así que el navegador ignoraba el display:none y el svg ocupaba su caja por defecto (300x150) empujando toda la interfaz hacia abajo. Era el único atributo style de todo el index.html.
- La validación del despliegue fue por HTTP y contenido, sin renderizar la página en un navegador: el hueco no se detectó antes de entregar.

# Soluciones implementadas
- Clase CSS en la hoja de estilos para ocultar el sprite sin ocupar layout. Verificado en el HTML servido por el host: cero atributos style=, clase icon-sprite presente, regla CSS servida, un solo proceso escuchando en 8081 con el md5 del binario nuevo y cámaras y grabación en marcha.

# Pendientes
- deuda-tecnica: añadir una comprobación renderizada (captura en navegador) a la lista de verificación de despliegues del frontend antes de darlos por buenos.

# Próximos pasos
1. Que el usuario recargue la interfaz y confirme que el hueco ha desaparecido.
2. Seguir con la fase 4 cuando el usuario la priorice.
