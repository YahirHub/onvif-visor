# Tarea 33 — Configuración central por cámara, retención y personas

## Estado
Completada y desplegada.

## Implementado
- Nueva pantalla /camaras/{id}/configuracion.
- Botón de configuración desde cada tarjeta de cámara.
- Grabación por eventos ONVIF.
- Grabación por persona NanoDet.
- Post-roll configurable.
- Modos: sin programación, continuo 24/7 y horario.
- Días y horas de programación.
- Acceso a analítica avanzada desde la misma pantalla.
- Métricas reales del filesystem.
- Retención automática configurable por cámara.
- Default de retención: activa al 90%.
- Limpieza hasta aproximadamente 5 puntos por debajo del umbral.
- Nunca elimina grabación activa.
- Borrado automático limpia también analítica y referencias de eventos.
- Persona mantiene la grabación activa mientras siga visible y usa post-roll al desaparecer.
- Persistencia cameras.json schema 2.

## Validación
- go test ./...: OK.
- go vet ./...: OK.
- esbuild app.js: OK.
- git diff --check: OK.
- Ruta SPA de configuración: OK.
- Smoke producción: OK.
- Servicio OpenRC: started.
- Disco al desplegar: 42%, por debajo del umbral 90%; no hubo borrado inmediato.
- README sin cambios.

## Despliegue
- Binario: /opt/onvif-visor/onvif-visor
- SHA256 nuevo: d974e6d5c66ddc9a2c1c862acf1ddb3e13c785730873e192c4f2626bea455044
- Backup: /opt/onvif-visor/backup-20260922-task33/onvif-visor
- SHA256 backup: 1afb590d13a6ca5a62aa9c187b2d8b55096fcfd880112e5d50f95173f7ae8c74
