# Pendiente — Reconocimiento y agrupación de personas / mascotas

## Estado
Pendiente. No implementar hasta solicitud explícita.

## Idea
Añadir una capa de identidad sobre la analítica actual sin sustituir NanoDet.

### Personas
- Detectar rostros dentro de personas detectadas.
- Generar embeddings faciales con un modelo ONNX especializado.
- Agrupar automáticamente rostros similares como desconocidos.
- Permitir que el usuario asigne nombres manualmente.
- Buscar apariciones por persona conocida.
- Mostrar identidad/apariciones en timeline y grabaciones.
- Mantener embeddings e información localmente.
- Controles de borrado, retención y acceso por tratarse de datos biométricos.

### Mascotas
- Partir de las clases dog/cat ya detectadas por NanoDet.
- Añadir reidentificación visual especializada para agrupar individuos.
- Permitir asignar nombres a grupos de mascotas.
- Tratar la identificación como probabilística, no infalible.

## Arquitectura propuesta
NanoDet -> detección de objeto -> modelo especializado de rostro/reidentificación -> embedding -> agrupación/comparación -> SQLite -> UI de identidades/apariciones.

## Notas
- NanoDet actual no identifica individuos.
- No se agregó código, modelo ni dependencia en esta tarea.
- Evaluar precisión, rendimiento, privacidad y consentimiento antes de implementar.
