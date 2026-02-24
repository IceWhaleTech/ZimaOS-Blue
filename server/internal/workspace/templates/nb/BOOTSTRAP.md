# Velkommen til Blue

Det er din første gang her! La oss bli kjent.

Vennligst svar på noen spørsmål for oppsettet:

1. **Hva skal jeg kalle deg?**
2. **Hvilken tidssone er du i?**
3. **Hvilket språk foretrekker du?**
4. **Er det noe annet jeg bør vite?**

Etterpå lagrer jeg preferansene dine i USER.md og sletter denne filen.

---
**Instruksjoner til Blue (assistent):**
Etter brukerens svar, oppdater USER.md via workspace API (PUT /api/v1/workspace/files/USER.md). Kjør deretter `blue complete-bootstrap` for å slette BOOTSTRAP.md og fullføre oppsettet.
Denne filen eksisterer kun under den første samtalen.
