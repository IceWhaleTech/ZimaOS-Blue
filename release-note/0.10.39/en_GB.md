# ZimaOS Blue v0.10.39

Released: 2026-04-09

Unified Research workflows, new Knowledge and Evolution workspaces, stronger conversation recovery, and improved document extraction and execution reliability.

## Highlights

- A more unified workflow for research, knowledge, and evolution tasks
- Better recovery when returning to active or waiting conversations
- Stronger reliability for document extraction and controlled execution

## New

- Added a unified `Research` entry while preserving mode-specific output
- Added a `Knowledge` workspace for compiled pages, lint health, and ask-and-archive jobs
- Added an `Evolution` console and local `blue audit` tooling for review and diagnostics

## Improved

- Improved conversation bootstrap so reopened chats restore more active state and pending approvals
- Improved Harness dataset workflows for more repeatable evaluation runs
- Improved context compression, failover handling, and localisation consistency

## Fixed

- Fixed provider catalog verification refresh loops
- Fixed repeated read-loop scenarios with artifact recovery handling
- Fixed malformed character repair during PDF text extraction

## Security

- Hardened command execution with `blue exec` and tier-aware sandbox routing

## Notes

If you encounter any issues, join the Zima community on Discord for support from over 43,000 members:

https://zimaboard.com/discord
