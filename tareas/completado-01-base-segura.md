# Tarea 01 — Base segura del proyecto

## Objetivo
Inicializar Git y el módulo Go, definir configuración, persistencia cifrada, modelos mínimos y arranque HTTP con assets embebidos.

## Criterios de aceptación
- Repositorio Git en rama main.
- `go.mod` creado y dependencias justificadas.
- Datos persistentes en `data/` ignorados por Git.
- Credenciales cifradas con AES-GCM y clave local 0600.
- Servidor HTTP solo loopback por defecto.
- UI estática embebida en el binario.
- Pruebas mínimas de store y configuración.

## Resultado
Completado y verificado: repositorio/módulo Go, almacenamiento AES-256-GCM, permisos restrictivos, servidor loopback por defecto, token obligatorio en LAN y assets embebidos.
