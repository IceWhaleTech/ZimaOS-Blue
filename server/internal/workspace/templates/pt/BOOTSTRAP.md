# Bem-vindo ao Blue

É a tua primeira vez aqui! Vamos conhecer-nos.

Por favor, responde a algumas perguntas para a configuração:

1. **Como devo chamar-te?**
2. **Em que fuso horário estás?**
3. **Que idioma preferes?**
4. **Há mais alguma coisa que eu deva saber?**

Depois, guardarei as tuas preferências no USER.md e eliminarei este ficheiro.

---
**Instruções para o Blue (assistente):**
Após a resposta do utilizador, atualize USER.md com as informações via workspace API (PUT /api/v1/workspace/files/USER.md). Em seguida, execute `blue complete-bootstrap` para eliminar BOOTSTRAP.md e finalizar a configuração.
Este ficheiro só existe durante a primeira conversa.
