# Tarea 15 — Integración Telegram

## Objetivo
Agregar un módulo Telegram configurable desde Integraciones para vincular chats, notificar eventos y pedir medios de cámaras.

## Alcance
- Validar y guardar token cifrado; eliminar token.
- Mostrar URL del bot obtenida con getMe.
- Código de 4 dígitos de un solo uso, válido 10 minutos; aceptar `/start <codigo>` y `/login <codigo>`.
- Polling Telegram con getUpdates y timeouts.
- Configurar notificaciones de eventos: foto o fragmento de vídeo y duración del fragmento.
- Comandos `/camaras`, `/foto <camara>` y `/video <camara>`.
- Enviar notificaciones sólo a chats vinculados y habilitados.
- UI Integraciones/Telegram responsiva.
- Pruebas unitarias del parser de comandos/códigos y smoke de API sin exponer token.

## Resultado
Implementado. La vista `Integraciones` expone Telegram como módulo. El token se comprueba con `getMe`, se cifra con la llave maestra existente y nunca se devuelve al navegador. El módulo puede eliminar token/chats/códigos. Los códigos son de 4 dígitos, bcrypt, un solo uso, duran 10 minutos y los intentos fallidos por chat están limitados. El poller admite `/start CODIGO`, `/login CODIGO`, `/camaras`, `/foto <camara>` y `/video <camara>`. Las alertas de evento pueden enviar foto o un MP4 de 3–10 s, con rate limit por cámara y una captura de vídeo simultánea. Se añadieron pruebas de códigos/configuración, parser de comandos y cliente Bot API contra servidor mock. La validación con un bot real queda condicionada a que el usuario introduzca su token; el panel realiza esa validación al guardarlo.
