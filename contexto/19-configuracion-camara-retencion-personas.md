# Contexto 19 — Configuración central por cámara, retención automática y grabación por personas

## Objetivo

Centralizar en una sola pantalla la configuración operativa de cada cámara e implementar la política automática de almacenamiento que hasta esta tarea no existía.

Antes de esta tarea:
- existía borrado manual individual y por rango;
- se calculaba el total físico de grabaciones;
- no existía una política automática que borrara archivos al llegar a un porcentaje de disco.

## Pantalla por cámara

Nueva ruta SPA:
- /camaras/{cameraID}/configuracion

La tarjeta de cada cámara ahora muestra un botón de configuración que abre esta pantalla.

La pantalla reúne:
- grabación por movimiento/eventos ONVIF;
- grabación por persona detectada por NanoDet;
- post-roll configurable de 1 a 600 segundos;
- grabación programada desactivada / continua 24/7 / por horario;
- días y horario;
- retención automática por porcentaje de disco;
- estado real del disco y espacio disponible;
- acceso a la configuración avanzada de analítica (clases, zonas, líneas y reglas);
- acceso directo a vista en vivo y grabaciones.

## Grabación por personas

Nuevo campo persistente por cámara:
- person_recording

Cuando está activo:
- un evento de clase person dispara inmediatamente el mismo grabador de la cámara;
- un monitor de detección reciente revisa NanoDet cada segundo;
- mientras NanoDet siga detectando una persona se prolonga stopAfter;
- cuando deja de verla, la grabación termina después del post-roll configurado;
- si existe una grabación continua, no se crea otro archivo: se reutiliza el archivo activo.

Al activar esta opción se garantiza que la clase person esté habilitada en la configuración NanoDet. Si NanoDet no está operativo, la API rechaza la activación para no dar una falsa sensación de funcionamiento.

## Retención automática

Nuevos campos persistentes por cámara:
- auto_retention
- retention_disk_percent

Valores por defecto:
- auto_retention = true
- retention_disk_percent = 90

Las cámaras existentes guardadas con el esquema anterior reciben estos defaults en memoria. Al siguiente guardado se persisten con schema version 2.

El servicio revisa el disco:
- inmediatamente al iniciar;
- cada 1 minuto;
- también después de guardar un cambio de retención.

Funcionamiento:
1. consulta el porcentaje real del filesystem que contiene recordings/;
2. identifica cámaras con auto_retention=true cuyo umbral ya fue alcanzado;
3. reúne únicamente MKV cerrados de esas cámaras;
4. ordena globalmente de más antiguo a más reciente;
5. elimina los más antiguos hasta recuperar aproximadamente 5 puntos porcentuales de margen;
6. nunca elimina el archivo actualmente activo;
7. el borrado reutiliza la misma ruta de limpieza que el borrado manual, por lo que elimina también analítica y referencias de eventos asociadas.

Ejemplo:
- cámara configurada a 90%;
- al llegar al 90% o más, entra en limpieza;
- objetivo aproximado: volver a 85%.

Los umbrales permitidos desde UI/API son 70%–95%.

El umbral es por cámara. Esto permite que una cámara pueda participar desde 85% y otra sólo desde 95%.

## Métricas de almacenamiento

GET /api/recordings/storage ahora también devuelve:
- disk_total_bytes
- disk_available_bytes
- disk_used_percent

Se usa Statfs sobre el filesystem de recordings/.

## Persistencia

cameras.json sube de schema version 1 a 2 al siguiente guardado.

Los secretos siguen cifrados igual que antes.

## Endpoints nuevos

- PUT /api/cameras/{id}/person-recording
- PUT /api/cameras/{id}/retention

Ruta UI nueva:
- GET /camaras/{id}/configuracion

## Validación

- gofmt: OK
- go test ./...: OK
- go vet ./...: OK
- esbuild app.js: OK
- git diff --check: OK
- pruebas nuevas:
  - persistencia de person recording y retención;
  - defaults de retención;
  - expiración de detección reciente de persona;
  - capacidad/porcentaje de filesystem;
  - ruta SPA de configuración.

## Situación del disco al desplegar

Antes del despliegue:
- filesystem /dev/sdb3
- uso: 42%
- recordings/: ~29.8 GB

Por tanto, el default 90% no provocó ninguna eliminación inmediata.

## Despliegue

Binario instalado:
- /opt/onvif-visor/onvif-visor
- SHA256: d974e6d5c66ddc9a2c1c862acf1ddb3e13c785730873e192c4f2626bea455044

Backup previo:
- /opt/onvif-visor/backup-20260922-task33/onvif-visor
- SHA256: 1afb590d13a6ca5a62aa9c187b2d8b55096fcfd880112e5d50f95173f7ae8c74

Smoke de producción:
- /api/health -> 200
- /camaras/cam-prueba/configuracion -> 200
- /app.js -> 200
- /styles.css -> 200
- /api/recordings/storage sin sesión -> 401 esperado
- servicio OpenRC -> started

README no se modificó.
