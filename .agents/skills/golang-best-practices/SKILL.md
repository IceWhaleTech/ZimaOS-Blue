---
name: golang-best-practices
description: |
  Use when writing or reviewing Golang code in this project.
  Triggers: database schema design, architecture decisions, high-availability implementation,
  code review, bug fixing, performance optimization, command execution, goroutine management.
  This project follows TDD (Test-Driven Development) - always write tests first.
---

# Golang Best Practices Skill

This skill provides coding standards and best practices for Golang development in this project.

## Development Mode: TDD (Test-Driven Development)

**This project strictly follows TDD. No exceptions.**

### The Iron Law

```
NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST
```

### Red-Green-Refactor Cycle

1. **RED** - Write a failing test first
2. **Verify RED** - Watch it fail (MANDATORY)
3. **GREEN** - Write minimal code to pass
4. **Verify GREEN** - Watch it pass (MANDATORY)
5. **REFACTOR** - Clean up while keeping tests green

### TDD Rules

- Write code before the test? **Delete it. Start over.**
- Test passes immediately? **Fix the test.**
- Can't explain why test failed? **Stop and investigate.**
- "I'll test after"? **No. Tests-after prove nothing.**

See `superpowers/skills/test-driven-development/SKILL.md` for complete TDD guidelines.

---

## Database Design

### 1. Backward Compatibility
- **Only add fields, never modify existing ones**
- Ensure schema migrations are always forward-compatible

### 2. SQL Injection Prevention
- Always use ORM or parameterized queries
- Never concatenate user input into SQL strings

### 3. Primary Key Requirements
- Every table MUST have at least one primary key
- Prefer using an `id` field as primary key

### 4. File Path Storage
When storing file paths in database:
- If prefix matching is needed, always store paths with trailing `/`
- **CAUTION**: `filepath.Base()` returns empty string for paths ending with `/`
- Handle this edge case explicitly in code

---

## Architecture Design

### 1. Trade-off Principle
> There's no best solution, only the most suitable one for the current situation.

When facing multiple choices:
1. Gather all evidence
2. Compare pros/cons and ROI
3. Choose the most appropriate solution
4. Document the decision rationale

### 2. Avoid Over-Engineering
**Say it three times: Don't over-design!**
- Don't implement features that "might" be needed in the future
- Consider future migration paths and upgrade strategies
- But don't implement them until actually needed

### 3. KISS (Keep It Simple, Stupid)
- Use simple, straightforward implementations
- Only add complexity when performance requires it
- Simple code is easier to maintain and debug

### 4. Avoid Premature Optimization
> Premature optimization is the root of all evil.

- Don't optimize before you have real performance data
- Optimization introduces complexity and potential bugs
- Wait until you have actual metrics

### 5. Measure Before Optimizing
Performance optimization levels (in order):
1. Algorithm optimization
2. Data structure optimization
3. I/O optimization
4. Concurrency optimization
5. Low-level optimization

**Always base optimization on real measurements, not assumptions.**

### 6. Occam's Razor - Decouple When Possible
- Prefer decoupling through third-party mechanisms
- Simplify problems to prevent cascading issues
- Example: Use file watching instead of API polling for config changes

### 7. Murphy's Law
> If you're worried something will happen, it's more likely to happen.

- Never ignore potential issues
- Address concerns proactively
- Add defensive code for edge cases

---

## High Availability Design

### 1. Degradation & Fallback
For scenarios with high error probability or critical availability impact:
- Implement basic degradation strategies
- Provide fallback logic
- Graceful degradation over hard failures

### 2. Caching Strategy
**When to use:**
- Long resource request times
- High request volume endpoints

**Implementation:**
- Consider cache invalidation strategy
- Set appropriate TTL
- Handle cache misses gracefully

### 3. Retry/Reconnect Mechanism
- Tolerate minor network fluctuations
- **MUST** control retry count
- **MUST** prevent infinite loops
- Use exponential backoff (see below)

### 4. Dual-Stack Network Mode
Priority order:
1. Default system preference
2. IPv6
3. IPv4 fallback

### 5. Dual-Engine Support (When Feasible)
- Support primary/backup switching
- Enable failover capabilities
- Example: AI features with both neural network (resource-limited) and LLM (full) versions

---

## Common Pitfalls

### 1. Recursion
- **Prefer loops over recursion** to prevent stack overflow
- If recursion is necessary, ensure proper termination conditions
- Consider tail-call optimization where applicable

### 2. Time Recording
- **Always use UTC time** for storage
- Timezone-independent storage enables proper UI rendering
- Convert to local time only at display layer

### 3. Channels
- Use sparingly
- Improper use leads to:
  - Deadlocks
  - Goroutine leaks
- Prefer other synchronization primitives when appropriate

### 4. Goroutines
- **Always use WaitGroup or goroutine pools**
- Prevent goroutine leaks
- Control memory usage
- Recommended: `sourcegraph/conc/pool`

### 5. Power Failure Considerations
Consider operation order for crash safety:
- Write data before index, or index before data?
- Delete index before data, or data before index?
- Prevent scenarios requiring manual intervention
- Never risk user data loss

### 6. Retry with Exponential Backoff
```go
// Use exponential backoff to prevent tight loops
// Recommended: rfyiamcool/backoff
```
- Add jitter to prevent thundering herd
- Set maximum retry count
- Set maximum backoff duration

### 7. Bug Fixing Protocol (TDD)
1. Write a failing test that reproduces the bug
2. Watch the test fail (confirms bug exists)
3. Implement the fix
4. Watch the test pass (confirms fix works)
5. Ensure all other tests still pass

**Never fix bugs without a failing test first.**

### 8. Local Service Calls
- Prefer `/var/run/casaos/x.url` over gateway
- Reduces network overhead
- Improves reliability
- TODO: Implement file watcher for config changes

---

## CI/CD

### Unit Testing Guidelines
- Keep test execution time short for fast CI/CD
- Maintain concise test cases
- Use path coverage methods with best cost-benefit ratio
- Focus on critical paths and edge cases
- **Every new function/method must have tests**
- **Tests must be written before implementation (TDD)**

---

## Recommended Libraries

| Purpose | Library | Notes |
|---------|---------|-------|
| Goroutine Pool | `sourcegraph/conc/pool` | Safe concurrent execution |
| Generic Utilities | `samber/lo` | Lodash-style helpers for Go |
| Unique ID | `orca-zhang/idgen` | Distributed ID generation |
| Local Cache | `orca-zhang/ecache` | High-performance local cache |
| Backoff | `rfyiamcool/backoff` | Exponential backoff implementation |
| Command Execution | `CasaOS-Common/utils/command` | Injection-safe command execution |
| ORM | `IceWhaleTech/zorm` | Database ORM library |

### Command Execution Safety
**CRITICAL**: When executing shell commands:
- Never use string concatenation with user input
- If input contains commas or spaces, use raw parameter execution
- Always validate and sanitize parameters before execution
- Use `OnlyExec` for untrusted input
- Injection risk is especially high with string types

---

## Checklist

Before submitting code, verify:

**TDD Compliance:**
- [ ] Every new function/method has a test
- [ ] Watched each test fail before implementing
- [ ] Each test failed for expected reason
- [ ] Wrote minimal code to pass each test
- [ ] All tests pass

**Code Quality:**
- [ ] Database changes are backward compatible
- [ ] SQL queries use parameterized statements
- [ ] No premature optimizations
- [ ] Retry mechanisms have backoff and limits
- [ ] Goroutines are properly managed
- [ ] Time values stored in UTC
- [ ] Power failure scenarios considered
- [ ] Command execution is injection-safe
