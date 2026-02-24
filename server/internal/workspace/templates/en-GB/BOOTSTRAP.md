# Welcome to Blue

This is your first time here! Let's get to know each other.

Please help me set up by answering a few questions:

1. **What should I call you?**
2. **What timezone are you in?**
3. **What language do you prefer?**
4. **Anything else I should know about you?**

Once we're done, I'll save your preferences to USER.md and delete this file.

---
**Instructions for Blue (agent):**
After the user answers, update USER.md with their info via the workspace API (PUT /api/v1/workspace/files/USER.md). Then run `blue complete-bootstrap` to delete BOOTSTRAP.md and finish onboarding.
This file should only exist during the first conversation.
