# Agents.md

## Language Rules

- All source code, comments, documentation, and new user-facing text must use `en_US`.
- Web UI text **must** route through i18n (`web/src/lib/i18n/messages/`); keep key structures aligned across locales (especially `en-us.ts` and `zh-hans.ts`). Never hardcode strings in Svelte components.
- Assistant conversations with the user use `zh_Hans` by default (concise, natural language). Use `en_US` only when necessary.
- Code comments: English only, explain **why** (never **how**).

## General Preferences

- Code style: functional-first, composition over inheritance, avoid OOP in TS/JS.
- New features: reuse/refactor existing code first — never pile on new code.
- Always follow KISS + DRY: smallest viable solution.
- When writing code, strictly follow ai-coding-discipline rules.
- Bad design? Refactor small issues immediately; for larger ones, add TODO + clear explanation.
- Architecture & design:
  - Start from first principles: clarify what is truly required before deciding how.
  - Avoid XY problems — validate the real issue and proactively suggest better alternatives.
  - Solve root causes, never workarounds. Refactor architecture if it no longer supports the need.
  - Immediately question unreasonable requirements or directions (no flattery, no blind agreement).
  - Reference ddia-principles and software-design-philosophy.
  - Tech choices: always recommend current industry best practices. Research first if unsure.

## Communication Rules

- Never silently make product, architecture, or public-API decisions.
- If anything is ambiguous, changes behavior/scope/structure, or has meaningful tradeoffs, ask the user before proceeding.
- Surface blockers, conflicts, or multiple reasonable paths clearly and ask for confirmation.

## Implementation Rules

- Prefer the smallest correct change.
- Keep early milestones narrow and end-to-end.
- Assume technical users. No excessive fault-tolerance or defensive UX unless explicitly required.
- Breaking changes are acceptable if they simplify the system.
- NEVER bypass GPG signing.
- Reuse existing i18n keys/namespaces before adding new ones.

## When To Ask The User

Ask before proceeding if any of the following is true:

- Requirement is ambiguous
- Decision affects public interfaces or naming
- Schema/protocol not fixed.
- Conflicts with existing work
- Multiple reasonable approaches with meaningful tradeoffs

## UI Component Library

- Web UI **MUST** use `shadcn-svelte` components (located in `web/src/lib/components/ui/`).
- Add new components with: `cd web && bunx shadcn-svelte@latest add <component-name>`

## Svelte 5 Syntax Requirements

All components must use runes mode:

- Props: `let { name, count = 0 }: { name: string; count?: number } = $props();`
- State: `let count = $state(0);`
- Derived: `let doubled = $derived(count * 2);`
- Effects: `$effect(() => { ... });`
- Two-way binding: `let { value = $bindable() } = $props();`
- Events: `onclick` (not `on:click`)
- Slots: `{@render children?.()}`
- Rest props: `...restProps`
- Always type props with TypeScript interfaces.
