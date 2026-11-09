# Fecha
2026-11-09

# Objetivo
Fijar la dirección visual del rediseño de onvif-visor en un DESIGN.md, tras validar el usuario el plan de rediseño y elegir empezar por la dirección visual.

# Decisiones tomadas
- Consola de videovigilancia, no dashboard SaaS. Diales Antislop: ENERGY 1, RHYTHM 1, MOTION 1.
- Tema oscuro único con motivo funcional (poca luz, el vídeo manda). Sin tema claro.
- Paleta heredada con un solo acento #50d0ad. Contrastes medidos: texto 18,34 y secundario nuevo #96a5b5 con 7,84 sobre el fondo.
- El antiguo --muted-2 #647180 queda prohibido en texto pequeño (3,96, no llega a 4,5). Ningún texto baja de 12 px ni de --text-2.
- Tipografía del sistema, sin fuentes descargadas. Monoespaciada solo para datos.
- Tres radios (6, 8, 12), espaciado en múltiplos de 4 y breakpoints oficiales 1100, 760 y 480.
- Navegación de siete secciones a cuatro: En directo, Grabaciones, Eventos y Ajustes.
- Sin eyebrows, sin degradados, sin pulsos. Estado siempre con palabra, no solo con color.
- Iconos en sprite SVG local. Se retira Lucide de unpkg en la fase de frontend.
- Cada decisión lleva su motivo en una línea, en el registro de DESIGN.md (regla R-31 de Antislop).

# Arquitectura actual
- Sin cambios de arquitectura en este turno. Solo documentación de diseño.

# Librerías usadas
- Ninguna nueva. Se elimina una dependencia externa prevista: Lucide por CDN.

# Archivos importantes modificados
- DESIGN.md (nuevo, 244 líneas, raíz del repo).
- contexto/25-design-md-direccion-visual.md (este archivo).

# Problemas encontrados
- El CSS actual usa diez valores de radio, unos 25 media queries con seis breakpoints y tamaños de letra hasta 0,52 rem.
- --muted-2 se usa en texto pequeño sin llegar al contraste mínimo.

# Soluciones implementadas
- DESIGN.md deja cerrados paleta con contrastes medidos, tipografía, radios, espaciado, breakpoints, componentes, textos y bloqueos de accesibilidad para la fase de frontend.

# Pendientes
- Aplicar DESIGN.md en la fase de frontend (tarea 4 de Alpine). Antes va el hub de streams (tarea 3).
- Decidir con el usuario si el live sigue en MJPEG compartido o salta a WebRTC.

# Próximos pasos
1. Hub de streams compartido por cámara, midiendo CPU y memoria antes y después.
2. Rediseño del frontend por módulos aplicando DESIGN.md pantalla a pantalla.
