# Web UI Style Guide

This document defines the default visual language for the Composia Web UI.

## Principles

- Prefer `shadcn-svelte` components and composition over one-off custom UI.
- Reuse the service workspace panel language across the rest of the product.
- Keep pages dense but readable for technical operators.
- Remove filler copy. Descriptions should only explain context that is not already obvious from the title or data.
- Optimize for desktop first, but keep all layouts usable on mobile browsers.

## Page Structure

- Use a single page shell with `max-w-6xl` for standard pages.
- Use the wide shell only for service workspace and other editor-style screens.
- Start each page with one primary header card.
- Keep page sections inside cards with a consistent border, radius, and background.
- Use `grid` layouts for overview dashboards and metadata summaries.

## Typography

- Page title: `text-2xl font-semibold tracking-tight`, `sm:text-3xl`.
- Page description: `text-sm leading-6 text-muted-foreground`.
- Section title: `text-base font-semibold tracking-tight`.
- Section description: `text-sm leading-6 text-muted-foreground`.
- Do not use decorative eyebrow labels above page titles.
- Metric labels should use normal case. Do not use all-caps labels for routine metadata.
- Body copy should default to `text-sm` unless a denser or larger size is clearly needed.

## Copy Rules

- Titles should be short noun phrases.
- Descriptions should be one sentence at most.
- Empty states should be direct and action-oriented.
- Avoid repeating data already visible in badges, tables, or metadata rows.
- Use repository UI text in `en_US`.

## Components

- Prefer these primitives first: `Card`, `Button`, `Input`, `Textarea`, `Badge`, `Table`, `Tabs`, `Alert`.
- Use `Alert` for errors and important status problems.
- Use `Badge` for status, counts, and compact metadata.
- Use `Table` for collection pages with stable columns.
- Use bordered list rows inside cards for recent activity and drill-down lists.

## Forms

- Labels stay above controls.
- Group related actions inside the same card.
- Inline helper text must be short and only used when a control has behavior that is not obvious.
- Primary save or run actions should be visually dominant; destructive actions should be clearly separated.

## Color And Surface

- Use token-driven surfaces only: `background`, `card`, `muted`, `accent`, `border`, `primary`, `destructive`.
- Cards should use slightly elevated contrast from the page background in both light and dark mode.
- Prefer subtle surface shifts over heavy shadows.
- Status colors should come from component variants instead of page-specific custom colors.

## Dark Mode

- Dark mode must preserve contrast for borders, muted text, editors, and hover states.
- Avoid pure black backgrounds; use tinted dark surfaces instead.
- Interactive rows must have a visible hover and selected state in both themes.
- Code and log surfaces should use the same border and background language as the rest of the UI.

## Responsive Behavior

- Stack header controls below titles on narrow screens.
- Avoid fixed multi-column layouts below `lg` unless the content is still readable.
- Count badges and metadata pills should wrap cleanly.
- Editor and workspace panels may stay vertically stacked on smaller screens.

## Do Not

- Do not introduce decorative marketing-style hero sections.
- Do not add long explanatory paragraphs to operational pages.
- Do not mix multiple heading scales for equivalent sections.
- Do not create ad-hoc colors or shadows when an existing token already fits.
