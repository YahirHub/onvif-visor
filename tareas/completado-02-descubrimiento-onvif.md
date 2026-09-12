# Tarea 02 — Descubrimiento y alta ONVIF

## Objetivo
Detectar dispositivos ONVIF por WS-Discovery, validar credenciales, consultar información/perfiles y obtener URI RTSP.

## Criterios de aceptación
- Descubrimiento con timeout y deduplicación.
- Alta desde descubrimiento y alta manual.
- No exponer credenciales en logs ni respuestas innecesarias.
- Compatibilidad con autenticación ONVIF soportada por la librería elegida.
- Tests de normalización/deduplicación.

## Resultado
Completado y verificado: WS-Discovery sobre interfaces activas con multicast, deduplicación, alta manual/descubierta, validación de credenciales, selección de perfil y URI RTSP sin credenciales persistidas.
