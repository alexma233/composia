# AGENTS.md

This file defines repository-wide collaboration rules for human contributors and coding agents.

## Language Rules

- All source code must use `en_US`.
- All code comments must use `en_US`.
- All documentation must use `en_US`.
- All user-facing repository text added to this project must use `en_US` unless a file already has an explicitly different requirement.
- For the Web UI, do not hardcode new user-facing copy in Svelte components when the project already routes UI text through i18n.
- When adding or changing Web UI user-facing text, update the i18n dictionaries in `web/src/lib/i18n/messages/` at the same time.
- Keep i18n key structures aligned across locales, especially `en-us.ts` and `zh-hans.ts`.
- All assistant conversation with the user use `zh_Hans` by default, feel free to use `en_US` if necessary. Never use uncommon/mechanical translation.
- Use concise language.

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
- NEVER bypass GPG signing for commits (e.g., do not use `--no-gpg-sign` or similar flags).
- Reuse existing i18n namespaces and message patterns before adding new translation keys.

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

- The Web UI MUST use `shadcn-svelte` components.
- Documentation: https://www.shadcn-svelte.com/
- Available components are located in `/home/alexma/Projects/composia/web/src/lib/components/ui/`.
- When adding new shadcn-svelte components, use `bunx` command:
  ```bash
  cd web && bunx shadcn-svelte@latest add <component-name>
  ```
- Prefer using existing shadcn-svelte components before creating custom ones.
- Refer to `style.md` for visual design guidelines.

## Svelte 5 Syntax Requirements

- All Svelte components must use Svelte 5 runes mode syntax.
- Use `$props()` instead of `export let` for component props:

  ```svelte
  <script>
    let { name, count = 0 }: { name: string; count?: number } = $props();
  </script>
  ```

- Use `$state()` for reactive state:

  ```svelte
  <script>
    let count = $state(0);
  </script>
  ```

- Use `$derived()` for computed values:

  ```svelte
  <script>
    let count = $state(0);
    let doubled = $derived(count * 2);
  </script>
  ```

- Use `$effect()` for side effects (replaces `$:` reactive statements):

  ```svelte
  <script>
    let count = $state(0);
    $effect(() => {
      console.log(count);
    });
  </script>
  ```

- Use `$bindable()` for two-way binding props:

  ```svelte
  <script>
    let { value = $bindable() } = $props();
  </script>
  ```

- Use `onclick` instead of `on:click` for event handlers.
- Use `{@render children?.()}` instead of `<slot />` for content projection.
- Use `...restProps` instead of `$$restProps` for spreading additional props.
- Component props must be typed with TypeScript interfaces.
