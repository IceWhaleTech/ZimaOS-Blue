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

### Planning with Files (planning-with-files)
Manus-style file-based planning for complex tasks. Creates persistent markdown files as "working memory on disk."

**Core Files:**
- `task_plan.md` — Track phases and progress
- `findings.md` — Store research and discoveries
- `progress.md` — Session log and test results

**Critical Rules:**
- Create plan file FIRST before any complex task
- The 2-Action Rule: After every 2 view/browser/search operations, save key findings to files
- Read plan before major decisions
- Update status after completing each phase
- Log ALL errors to prevent repetition
- 3-Strike Error Protocol: Diagnose → Alternative Approach → Broader Rethink → Escalate

**When to Use:**
- Multi-step tasks (3+ steps)
- Research tasks
- Building/creating projects
- Tasks spanning many tool calls

**Usage:** Read `.agent/skills/planning-with-files/.agent/skills/planning-with-files/SKILL.md`

### UI/UX Pro Max (ui-ux-pro-max-skill)
Comprehensive UI/UX design intelligence with 50+ styles, 97 color palettes, 57 font pairings, and 9 technology stacks.

**Supported Stacks:** React, Next.js, Vue, Svelte, SwiftUI, React Native, Flutter, Tailwind, shadcn/ui

**Priority Categories:**
1. Accessibility (CRITICAL) - Color contrast, focus states, ARIA labels
2. Touch & Interaction (CRITICAL) - Touch targets, loading states, error feedback
3. Performance (HIGH) - Image optimization, reduced motion
4. Layout & Responsive (HIGH) - Viewport, font sizes, z-index
5. Typography & Color (MEDIUM) - Line height, font pairing
6. Animation (MEDIUM) - Duration timing, transform performance
7. Style Selection (MEDIUM) - Style matching, consistency
8. Charts & Data (LOW) - Chart types, color guidance

**Workflow:**
1. Analyze user requirements (product type, style, industry, stack)
2. Generate design system with `--design-system` flag (REQUIRED)
3. Supplement with detailed domain searches as needed
4. Get stack-specific guidelines (default: html-tailwind)

**Usage:** Read `.agent/skills/ui-ux-pro-max-skill/.claude/skills/ui-ux-pro-max/SKILL.md`

## How to Use Skills

When working on tasks, reference the appropriate skill:

1. **For brainstorming/design:** Read `.agent/skills/superpowers/skills/brainstorming/SKILL.md`
2. **For planning:** Read `.agent/skills/superpowers/skills/writing-plans/SKILL.md`
3. **For TDD:** Read `.agent/skills/superpowers/skills/test-driven-development/SKILL.md`
4. **For debugging:** Read `.agent/skills/superpowers/skills/systematic-debugging/SKILL.md`
5. **For Golang best practices:** Read `.agent/skills/golang-best-practices/SKILL.md`
6. **For file-based planning:** Read `.agent/skills/planning-with-files/.agent/skills/planning-with-files/SKILL.md`
7. **For UI/UX design:** Read `.agent/skills/ui-ux-pro-max-skill/.claude/skills/ui-ux-pro-max/SKILL.md`

## Task Execution Rules

**IMPORTANT: Complete tasks ONE BY ONE**

When working on multiple tasks or a complex task with multiple steps:

1. **Sequential Execution** - Complete one task fully before starting the next
2. **No Parallel Task Switching** - Do not jump between incomplete tasks
3. **Mark Progress** - Update task status immediately after completion
4. **Verify Before Moving On** - Ensure current task is truly complete before proceeding
5. **Large Document Chunking** - When writing large documents (>200 lines or >5KB), MUST split into multiple write operations to prevent failures. Write section by section, verify each write succeeds before continuing
6. **Route Registration Checkpoint** - When implementing frontend and backend code together, add a checkpoint to verify all backend routes are properly registered before proceeding. Never skip this checkpoint - unregistered routes cause 404 errors
7. **Internationalization (i18n)** - When adding i18n content, English locale is sufficient. Do not add other languages unless explicitly requested

This ensures:
- Higher quality output
- Better tracking of progress
- Easier debugging if issues arise
- Clear accountability for each step
