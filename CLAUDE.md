# ZimaOS-Echo Project

## Skills Reference

This project uses the following skills from `.agent/skills/`:

### Superpowers (obra/superpowers)
A complete software development workflow with composable skills.

**Available Skills:**
- `brainstorming` - Socratic design refinement
- `writing-plans` - Detailed implementation plans
- `executing-plans` - Batch execution with checkpoints
- `test-driven-development` - RED-GREEN-REFACTOR cycle
- `systematic-debugging` - 4-phase root cause process
- `verification-before-completion` - Ensure fixes are complete
- `subagent-driven-development` - Fast iteration with two-stage review
- `dispatching-parallel-agents` - Concurrent subagent workflows
- `requesting-code-review` - Pre-review checklist
- `receiving-code-review` - Responding to feedback
- `using-git-worktrees` - Parallel development branches
- `finishing-a-development-branch` - Merge/PR decision workflow
- `writing-skills` - Create new skills

**Usage:** Read skill files from `.agent/skills/superpowers/skills/<skill-name>/SKILL.md`

### Golang Best Practices (golang-best-practices)
Coding standards and best practices for Golang development.

**Triggers:**
- Database schema design
- Architecture decisions
- High-availability implementation
- Code review
- Bug fixing
- Performance optimization
- Command execution
- Goroutine management

**Key Points:**
- This project follows TDD (Test-Driven Development)
- Always write tests first
- No production code without a failing test

**Usage:** Read `.agent/skills/golang-best-practices/SKILL.md`

## How to Use Skills

When working on tasks, reference the appropriate skill:

1. **For brainstorming/design:** Read `.agent/skills/superpowers/skills/brainstorming/SKILL.md`
2. **For planning:** Read `.agent/skills/superpowers/skills/writing-plans/SKILL.md`
3. **For TDD:** Read `.agent/skills/superpowers/skills/test-driven-development/SKILL.md`
4. **For debugging:** Read `.agent/skills/superpowers/skills/systematic-debugging/SKILL.md`
5. **For Golang best practices:** Read `.agent/skills/golang-best-practices/SKILL.md`
