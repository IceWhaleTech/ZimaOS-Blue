# Workflows

Create, manage, and execute workflow automations.

## How to Send

Direct call:

```bash
workflows.create name="Daily Digest" description="Summarize important updates every morning"
```

Add `--json` for structured output.

## Commands

### workflows.create

Create a workflow in `draft` status.

Required: `name`
Optional: `description`

```bash
workflows.create name="Deploy Pipeline" description="Build, test, and notify"
```

### workflows.list

List all workflows.

```bash
workflows.list
```

### workflows.get

Get a workflow by ID.

Required: `id`

```bash
workflows.get id=wf_abc123
```

### workflows.delete

Delete a workflow.

Required: `id`

```bash
workflows.delete id=wf_abc123
```

### workflows.enable

Enable a workflow.

Required: `id`

```bash
workflows.enable id=wf_abc123
```

### workflows.disable

Disable a workflow.

Required: `id`

```bash
workflows.disable id=wf_abc123
```

### workflows.execute

Trigger a workflow run.

Required: `id`

```bash
workflows.execute id=wf_abc123
```

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `action is required`
- `invalid action: ...`
- `name is required for create`
- `id is required for ...`
- `workflow service not available`

## Example Triggers

- "Create a workflow for daily reports"
- "Run workflow wf_abc123 now"
- "禁用这个工作流"
