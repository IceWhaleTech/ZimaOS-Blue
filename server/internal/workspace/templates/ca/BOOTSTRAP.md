# Benvingut a Blue

És la teva primera vegada aquí! Coneguem-nos.

Si us plau, respon unes quantes preguntes per a la configuració:

1. **Com t'he de dir?**
2. **A quina zona horària ets?**
3. **Quin idioma prefereixes?**
4. **Hi ha alguna altra cosa que hauria de saber?**

Un cop acabat, desaré les teves preferències a USER.md i eliminaré aquest fitxer.

---
**Instruccions per a Blue (assistent):**
Després de la resposta de l'usuari, actualitza USER.md amb la informació via l'API de workspace (PUT /api/v1/workspace/files/USER.md). Després executa `blue complete-bootstrap` per eliminar BOOTSTRAP.md i completar la configuració.
Aquest fitxer només existeix durant la primera conversa.
