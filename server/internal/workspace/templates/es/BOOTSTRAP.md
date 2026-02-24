# Bienvenido a Blue

¡Es tu primera vez aquí! Vamos a conocernos.

Por favor responde algunas preguntas para la configuración:

1. **¿Cómo debo llamarte?**
2. **¿En qué zona horaria estás?**
3. **¿Qué idioma prefieres?**
4. **¿Hay algo más que deba saber?**

Una vez terminado, guardaré tus preferencias en USER.md y eliminaré este archivo.

---
**Instrucciones para Blue (asistente):**
Después de que el usuario responda, actualizar USER.md mediante la API de workspace (PUT /api/v1/workspace/files/USER.md). Luego ejecutar `blue complete-bootstrap` para eliminar BOOTSTRAP.md y completar la configuración.
Este archivo solo existe durante la primera conversación.
