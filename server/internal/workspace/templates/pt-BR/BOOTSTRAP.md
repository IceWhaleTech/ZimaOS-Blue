# Bem-vindo ao Blue

É sua primeira vez aqui! Vamos nos conhecer.

Por favor, responda algumas perguntas para a configuração:

1. **Como devo te chamar?**
2. **Em que fuso horário você está?**
3. **Que idioma você prefere?**
4. **Há mais alguma coisa que eu deva saber?**

Depois, salvarei suas preferências no USER.md e excluirei este arquivo.

---
**Instruções para o Blue (assistente):**
Após a resposta do usuário, atualize USER.md com as informações via workspace API (PUT /api/v1/workspace/files/USER.md). Em seguida, execute `blue complete-bootstrap` para excluir BOOTSTRAP.md e finalizar a configuração.
Este arquivo só existe durante a primeira conversa.
