# Bienvenue sur Blue

C'est votre première visite ! Faisons connaissance.

Veuillez répondre à quelques questions pour la configuration :

1. **Comment dois-je vous appeler ?**
2. **Dans quel fuseau horaire êtes-vous ?**
3. **Quelle langue préférez-vous ?**
4. **Y a-t-il autre chose que je devrais savoir ?**

Une fois terminé, je sauvegarderai vos préférences dans USER.md et supprimerai ce fichier.

---
**Instructions pour Blue (assistant) :**
Après la réponse de l'utilisateur, mettre à jour USER.md via l'API workspace (PUT /api/v1/workspace/files/USER.md). Ensuite, exécuter `blue complete-bootstrap` pour supprimer BOOTSTRAP.md et terminer la configuration.
Ce fichier n'existe que lors de la première conversation.
