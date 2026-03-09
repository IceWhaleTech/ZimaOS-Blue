# Chat Chain Gap Report

- Generated at: `2026-03-04T11:27:52Z`
- Repo root: `/Users/orca/Documents/GitHub/ZimaOS-Blue`
- Summary: total=23, ok=23, missing=0, warning=0

## routes_missing

| check | status | details |
|---|---|---|
| chat_routes_registered | ok | - |
| session_routes_registered | ok | - |
| gateway_routes_registered | ok | - |

## gateway_methods_missing

| check | status | details |
|---|---|---|
| browser.request | ok | - |
| chat.abort | ok | - |
| chat.send | ok | - |
| hooks.wake | ok | - |
| sessions.list | ok | - |
| sessions.reset | ok | - |

## settings_config_missing

| check | status | details |
|---|---|---|
| smart_tool_selection | ok | - |
| smart_skill_selection | ok | - |
| skill_selector_mode | ok | - |
| skill_selector_confidence_threshold | ok | - |
| prompt_policy_version | ok | - |
| prompt_policy_profile | ok | - |
| agent_loop_policy_max_tool_rounds | ok | - |
| agent_loop_policy_max_auto_continue | ok | - |
| agent_loop_policy_pseudo_tool_call_budget | ok | - |
| agent_loop_policy_action_pledge_budget | ok | - |
| agent_loop_policy_missing_todo_budget | ok | - |
| agent_loop_policy_pending_todo_budget | ok | - |

## hook_lifecycle_missing

| check | status | details |
|---|---|---|
| register_hook_available | ok | - |
| hook_trigger_lifecycle_wired | ok | trigger call sites: server/internal/bootstrap/routes.go |

