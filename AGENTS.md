# AGENTS.md

This file defines repository-wide collaboration rules for human contributors and coding agents.

## Language Rules

- All source code must use `en_US`.
- All code comments must use `en_US`.
- All documentation must use `en_US`.
- All user-facing repository text added to this project must use `en_US` unless a file already has an explicitly different requirement.
- All assistant conversation with the user must use `zh_Hans`.

## Communication Rules

- Do not silently make product or architecture decisions when the requirement is unclear.
- If a decision changes behavior, scope, structure, or public API, ask the user before proceeding.
- If a blocker, ambiguity, or conflict is found, ask the user before choosing a direction.
- Surface tradeoffs clearly and ask for confirmation when more than one reasonable implementation path exists.

## Implementation Rules

- Prefer the smallest correct change.
- Align implementation with `plan.md` unless the user explicitly asks to change the plan.
- Do not introduce undocumented behavior that conflicts with `plan.md`.
- Keep early milestones narrow and end-to-end instead of partially implementing many systems at once.
- Do not add excessive fault-tolerance, hand-holding, or defensive UX unless explicitly required.
- Assume operators and contributors are technical users by default.
- Do not preserve old APIs for compatibility unless the user explicitly requires it.
- Breaking changes are acceptable when they simplify the system and stay aligned with `plan.md`.

## Current Delivery Strategy

- Build the controller and agent foundation first.
- Finish config loading, SQLite initialization, and the first controller-agent heartbeat before larger features.
- Implement task execution, repo parsing, secrets, backups, and migration only after the platform skeleton is working.

## When To Ask The User

Ask before proceeding if any of the following is true:

- a requirement is ambiguous
- a naming choice affects public interfaces
- a schema or protocol decision is not already fixed in `plan.md`
- an implementation conflicts with existing work in the repository
- there are multiple reasonable approaches with meaningful tradeoffs

## Practical Default

If the next step is clear and already consistent with `plan.md`, implement it directly.
If it is not clear, stop and ask a focused question in `zh_Hans`.

## UI Component Library

- The Web UI uses `shadcn-svelte` components.
- Documentation: https://www.shadcn-svelte.com/
- Available components are located in `/home/alexma/Projects/composia/web/src/lib/components/ui/`.
- Prefer using existing shadcn-svelte components before creating custom ones.
- Refer to `style.md` for visual design guidelines.
