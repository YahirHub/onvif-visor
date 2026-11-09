# Fecha
2026-11-09

# Objetivo
Analizar a detalle onvif-visor clonado de YahirHub/onvif-visor en el sandbox de Alpine y proponer mejoras, optimizaciones y un rediseño total, siguiendo Ponytail y Antislop. En este turno no se reescribe código: se entrega el plan para validarlo con el usuario.

# Decisiones tomadas
- El usuario decide rediseñar todo el proyecto.
- Durante todo el proyecto se siguen las reglas Ponytail y NoSlop/Antislop de ThowiLabs MCP.
- Se usa siempre el contexto, el journal de decisiones y la tasklist de Alpine, porque varios agentes trabajan en el mismo proyecto.
- Propuesta a validar: rediseño total por fases conservando el núcleo que ya funciona (ONVIF, grabación con FFmpeg y persistencia), en lugar de una reescritura desde cero sin rollback.

# Arquitectura actual
- Monobinario Go 1.24, UI embebida, sin build frontend. Unas 19.300 líneas de código y 100 funciones de test en 28 archivos.
- internal/onvif: descubrimiento WS-Discovery, perfiles, eventos PullPoint.
- internal/media: un proceso FFmpeg por espectador en live MJPEG, grabación con -c copy, snapshots, clips y compresión.
- internal/server: App actúa como objeto central con rutas (unas 60), sesiones, watchers, eventos en memoria, scheduler, retención y Telegram (telegram_integration.go, 945 líneas).
- internal/analytics: NanoDet Plus ONNX de 4,6 MB embebido, multiclase, zonas, líneas y tracking. Store SQLite propio, separado del de cuentas.
- internal/store: cámaras en cameras.json cifrado con AES-256-GCM y master.key en el mismo directorio.
- internal/account: usuarios, sesiones y Telegram en SQLite.
- web: index.html de 732 líneas, styles.css de 1.368 y app.js de 3.834 con 174 funciones y estado global único.

# Librerías usadas
- github.com/0x524a/onvif-go, github.com/yalue/onnxruntime_go, golang.org/x/crypto y modernc.org/sqlite. Solo 4 dependencias directas: punto fuerte Ponytail que debe conservarse.

# Archivos importantes modificados
- Ningún archivo de código en este turno. Solo se crea este contexto.
- Clon local en /workspace/onvif-visor, rama main, commit edb6b0e.

# Problemas encontrados
- Seguridad sólida en lo básico (bcrypt, AES-GCM, crypto/rand, cabeceras, timeouts, LAN exige usuario), pero la CSP es script-src self y index.html carga Lucide desde unpkg: el script externo queda bloqueado y los iconos caen a glifos Unicode como ▣ o ◫. Depender de un CDN contradice un visor local que debe funcionar sin Internet.
- Rendimiento: cada espectador de live lanza su propio FFmpeg que decodifica y recodifica MJPEG (8 fps, 960 px, máximo 4 por cámara). Multivista multiplica procesos por la misma cámara. La analítica abre además su propio stream RTSP por cámara.
- Polling en varios frentes: scheduler cada 2 s, grabación por persona cada 1 s, Telegram cada 1 s y en el frontend temporizadores de eventos y cámaras, en vez de un mecanismo de eventos o SSE.
- Eventos en memoria limitados a 200 y con copia completa del slice en cada evento nuevo. Se pierden al reiniciar.
- Alcance: AGENTS.md dice que el MVP no incluye analíticas propias ni reconocimiento, pero el proyecto ya incluye analítica multiclase, zonas y tracking, y deja pendiente reconocimiento de personas y mascotas. La arquitectura preferida de AGENTS.md ya no describe los paquetes account, analytics, service y telegram.
- Configuración por cámara fragmentada en endpoints separados: nombre, grabación, grabación por persona, retención, horario y analítica.
- Sin CI, sin tags ni releases y sin Dockerfile. Cuatro tests de internal/server fallan sin FFmpeg instalado porque dependen del binario real: la suite no es hermética.
- Frontend Antislop: eyebrows en mayúsculas en casi cada vista y diálogo, shell de dashboard genérico con sidebar, Inter declarado pero nunca cargado, tema solo oscuro sin alternativa ni motivo escrito, y radios y tarjetas uniformes.
- README de 357 líneas que mezcla manual, arquitectura y diario. Sin DESIGN.md que fije identidad, paleta y diales de energía, ritmo y movimiento.

# Soluciones implementadas
- Ninguna en código. Queda el análisis y la propuesta de rediseño por fases.

# Propuesta de rediseño
- Fase 0, dirección: crear DESIGN.md con identidad propia de consola de videovigilancia, 2 o 3 colores núcleo y un acento, tipografía cargada de verdad o del sistema con motivo escrito, y diales ENERGY 1, RHYTHM 1 y MOTION 1 propios de una herramienta de monitoreo.
- Fase 1, streams: hub único por cámara que decodifica una vez y reparte frames a todos los espectadores, snapshots y analítica. Un FFmpeg por cámara, no por espectador.
- Fase 2, frontend: partir app.js en módulos ES nativos por vista sin bundler, iconos SVG locales inline y cero CDN, una sola pantalla principal cuyo foco es el vídeo, y estados vacíos, de error y de carga reales por vista.
- Fase 3, servidor: partir el App central por dominios (cámaras, grabaciones, analítica, Telegram) sin capas vacías, unificar la configuración de cámara en un solo recurso y mover eventos a persistencia con SSE en lugar de polling.
- Fase 4, datos y entrega: decidir una sola persistencia o documentar el reparto JSON y SQLite con su motivo, clave maestra separable de los datos, tests que no dependan de FFmpeg real, CI mínima con gofmt, vet y test, y tags con releases.

# Pendientes
- Validar esta propuesta con el usuario, en especial si el rediseño incluye cambiar el live de MJPEG a otra tecnología o se conserva por compatibilidad.
- Decidir si la analítica inteligente sigue en el mismo binario o pasa a ser opcional y desactivable por completo en compilación y en ejecución.

# Próximos pasos
1. Validar el plan y la dirección visual con el usuario.
2. Escribir DESIGN.md y el sistema visual antes de tocar la UI.
3. Implementar el hub de streams y medir CPU y memoria antes y después.
4. Rehecer el frontend por vistas manteniendo la API o versionándola con motivo.
