<script lang="ts">
	import { cn } from "$lib/utils";
	import type { Snippet } from "svelte";
	import type { HTMLAnchorAttributes } from "svelte/elements";

	let {
		class: className,
		href,
		children,
		page,
		active = false,
		...restProps
	}: HTMLAnchorAttributes & {
		children?: Snippet<[]>;
		page: number;
		active?: boolean;
	} = $props();
</script>

<a
	{href}
	class={cn(
		"inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors",
		"h-9 w-9",
		"border border-border/70 bg-background/95 hover:bg-accent hover:text-accent-foreground",
		active && "bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground",
		className,
	)}
	aria-current={active ? "page" : undefined}
	{...restProps}
>
	{@render children?.()}
	{page}
</a>