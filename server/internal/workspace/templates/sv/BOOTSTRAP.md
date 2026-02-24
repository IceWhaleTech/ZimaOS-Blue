# Välkommen till Blue

Det här är första gången du är här! Låt oss lära känna varandra.

Svara på några frågor för konfigurationen:

1. **Vad ska jag kalla dig?**
2. **Vilken tidszon är du i?**
3. **Vilket språk föredrar du?**
4. **Finns det något annat jag borde veta?**

När vi är klara sparar jag dina inställningar i USER.md och tar bort den här filen.

---
**Instruktioner för Blue (assistent):**
Efter användarens svar, uppdatera USER.md via workspace API (PUT /api/v1/workspace/files/USER.md). Kör sedan `blue complete-bootstrap` för att ta bort BOOTSTRAP.md och slutföra konfigurationen.
Den här filen finns bara under det första samtalet.
