# Tarea 04 — Eventos ONVIF y grabación

## Objetivo
Consumir eventos ONVIF (PullPoint cuando esté disponible), detectar eventos de movimiento/alarma compatibles y disparar grabaciones RTSP.

## Criterios de aceptación
- Suscripción/polling con cancelación y reintentos acotados.
- Registro de eventos sin secretos.
- Grabación a `recordings/<camera-id>/` con nombres seguros.
- Copia de stream sin recodificar cuando FFmpeg lo permita.
- Post-roll configurable y prevención de procesos duplicados.
- Estado de grabación visible por API/UI.

## Resultado
Completado y verificado: PullPoint con reintentos, reporte de eventos en UI, filtrado de estados activos/inactivos, grabación MKV con copia de stream y post-roll extensible.
