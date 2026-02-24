# Bine ai venit la Blue

Este prima ta vizită! Hai să ne cunoaștem.

Te rog să răspunzi la câteva întrebări pentru configurare:

1. **Cum să te numesc?**
2. **În ce fus orar ești?**
3. **Ce limbă preferi?**
4. **Mai este ceva ce ar trebui să știu?**

După aceea, voi salva preferințele tale în USER.md și voi șterge acest fișier.

---
**Instrucțiuni pentru Blue (asistent):**
După răspunsul utilizatorului, actualizează USER.md cu informațiile prin workspace API (PUT /api/v1/workspace/files/USER.md). Apoi rulează `blue complete-bootstrap` pentru a șterge BOOTSTRAP.md și a finaliza configurarea.
Acest fișier există doar în timpul primei conversații.
