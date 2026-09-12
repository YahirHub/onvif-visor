# Fecha
2026-09-12

# Objetivo
Crear un visor ONVIF local escrito en Go que descubra cámaras, permita agregarlas, muestre vídeo en vivo y grabe en respuesta a eventos ONVIF.

# Decisiones tomadas
- Aplicación monobinaria Go con UI web embebida.
- FFmpeg permitido como dependencia externa para RTSP->MJPEG y grabación.
- Perfil T será la referencia principal; se mantendrá compatibilidad pragmática con dispositivos Profile S mientras siga siendo necesario.
- Descubrimiento vía WS-Discovery en la LAN.
- Servicio HTTP ligado a loopback por defecto.
- Credenciales de cámaras cifradas en persistencia local.

# Arquitectura actual
Proyecto nuevo. Estructura objetivo definida en AGENTS.md.

# Librerías usadas
Pendiente de selección final tras revisar mantenimiento y APIs. Se prioriza una biblioteca ONVIF activa en Go y standard library para el resto.

# Archivos importantes modificados
- AGENTS.md
- contexto/01-contexto-inicial.md

# Problemas encontrados
Los navegadores no reproducen RTSP directamente; se requiere un puente de medios.

# Soluciones implementadas
Se decidió usar FFmpeg como proceso externo para convertir el stream a MJPEG de baja latencia y para grabación sin recodificar cuando sea posible.

# Pendientes
- Inicializar Git y módulo Go.
- Confirmar dependencia ONVIF.
- Implementar descubrimiento y alta.
- Implementar live view.
- Implementar eventos y grabación.
- Implementar UI.
- Pruebas, documentación y empaquetado.

# Próximos pasos
Crear tareas persistentes e iniciar la base del proyecto.
