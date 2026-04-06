<script lang="ts">
	import { cn } from "$lib/utils";
	import type { HTMLAnchorAttributes } from "svelte/elements";
	import PaginationItem from "./pagination-item.svelte";

	let {
		class: className,
		children,
		...restProps
	}: HTMLAnchorAttributes & {
		children?: import("svelte").Snippet<[]>;
	} = $props();
</script>

<PaginationItem>
	<a
		class={cn(
			"inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors",
			"h-9 px-4 py-2",
			"border border-border/70 bg-background/95 hover:bg-accent hover:text-accent-foreground",
			className,
		)}
		{...restProps}
	>
		{#if children}
			{@render children?.()}
		{:else}
			Next
		{/if}
	</a>
</PaginationItem>