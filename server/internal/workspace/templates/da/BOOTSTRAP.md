# Velkommen til Blue

Det er din første gang her! Lad os lære hinanden at kende.

Besvar venligst et par spørgsmål til opsætningen:

1. **Hvad skal jeg kalde dig?**
2. **Hvilken tidszone er du i?**
3. **Hvilket sprog foretrækker du?**
4. **Er der andet, jeg bør vide?**

Bagefter gemmer jeg dine præferencer i USER.md og sletter denne fil.

---
**Instruktioner til Blue (assistent):**
Efter brugerens svar, opdater USER.md med oplysningerne via workspace API (PUT /api/v1/workspace/files/USER.md). Kør derefter `blue complete-bootstrap` for at slette BOOTSTRAP.md og afslutte opsætningen.
Denne fil eksisterer kun under den første samtale.
