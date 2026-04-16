# ZimaOS-Blue Project

This is the canonical cross-agent instructions document for this repository.

## Skills Reference

This project uses the following skills from `.agents/skills/` and `.agents/skill/`:

### Mobile Toggle UX Optimization (.agents/skill)
A focused design skill for optimizing crowded mobile toggle UI and making `Auto` mode clearly visible.

**Usage:** Read `.agents/skill/mobile-toggle-ux/SKILL.md`

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

**Usage:** Read skill files from `.agents/skills/superpowers/skills/<skill-name>/SKILL.md`

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

**Usage:** Read `.agents/skills/golang-best-practices/SKILL.md`

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

**Usage:** Read `.agents/skills/planning-with-files/skills/planning-with-files/SKILL.md`

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

**Usage:** Read `.agents/skills/ui-ux-pro-max-skill/.claude/skills/ui-ux-pro-max/SKILL.md`

### Product Skills (assets/skills/)
Built-in skills shipped with the product. Located in `assets/skills/<name>/SKILL.md`.

## How to Use Skills

When working on tasks, reference the appropriate skill:

1. **For brainstorming/design:** Read `.agents/skills/superpowers/skills/brainstorming/SKILL.md`
2. **For planning:** Read `.agents/skills/superpowers/skills/writing-plans/SKILL.md`
3. **For TDD:** Read `.agents/skills/superpowers/skills/test-driven-development/SKILL.md`
4. **For debugging:** Read `.agents/skills/superpowers/skills/systematic-debugging/SKILL.md`
5. **For Golang best practices:** Read `.agents/skills/golang-best-practices/SKILL.md`
6. **For file-based planning:** Read `.agents/skills/planning-with-files/skills/planning-with-files/SKILL.md`
7. **For UI/UX design:** Read `.agents/skills/ui-ux-pro-max-skill/.claude/skills/ui-ux-pro-max/SKILL.md`
8. **For product skills:** Read `assets/skills/<name>/SKILL.md`

## Task Execution Rules

**IMPORTANT: Complete tasks ONE BY ONE**

When working on multiple tasks or a complex task with multiple steps:

1. **Sequential Execution** - Complete one task fully before starting the next
2. **No Parallel Task Switching** - Do not jump between incomplete tasks
3. **Mark Progress** - Update task status immediately after completion
4. **Verify Before Moving On** - Ensure current task is truly complete before proceeding
5. **Large Document Chunking** - When writing large documents (>200 lines or >5KB), MUST split into multiple write operations to prevent failures. Write section by section, verify each write succeeds before continuing
6. **Route Registration Checkpoint** - When implementing frontend and backend code together, add a checkpoint to verify all backend routes are properly registered before proceeding. Never skip this checkpoint - unregistered routes cause 404 errors
7. **Internationalization (i18n)** - When adding i18n content, English & Chinese locale is sufficient. Do not add other languages unless explicitly requested
8. **Model Source of Truth** - Never trust upstream `response.model`/`resp.Model` as routing truth. Use request model + locally routed/known model list (`resolved model`, provider model inventory) as the authoritative source for persistence, metrics, retries, and follow-up rounds.
9. **Execution Framework Changes** - If a task modifies the execution framework, use Blue Harness to add or update the relevant datasets first, then validate the candidate through the gate-first cutover workflow. Prefer selector/execution/budget gates and cutover-readiness evidence over ad hoc checks, and do not open a PR until the data shows quality is not regressing and the candidate is confirmed ready to cut over.
10. **GitHub Information Retrieval High Availability** - For GitHub-hosted repository file/document/content retrieval, use the project-standard 4-source chain in this exact order: `raw.githubusercontent.com` -> `raw.gitmirror.com` -> `cdn.jsdelivr.net/gh` -> `ghproxy.com` (proxying `raw.githubusercontent.com`). Do not merge a GitHub retrieval path that relies on fewer than these 4 sources unless the task explicitly documents why the content type cannot use the standard chain and defines an equally explicit fallback list.
11. **HF/MS Model Retrieval High Availability** - For model retrieval from Hugging Face (`hf`) or ModelScope (`ms`), use the project-standard 3-source chain in this exact order: `modelscope.cn/models/...` -> `huggingface.co/...` -> `hf-mirror.com/...`. The same model identity, filename, and revision/path must be mapped across all 3 sources, and the downloader must verify integrity before treating any source as a successful replacement.

This ensures:
- Higher quality output
- Better tracking of progress
- Easier debugging if issues arise
- Clear accountability for each step
