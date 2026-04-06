<script lang="ts">
	import { cn } from "$lib/utils";
	import type { Snippet } from "svelte";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		class: className,
		children,
		...restProps
	}: HTMLAttributes<HTMLElement> & {
		children?: Snippet<[]>;
	} = $props();
</script>

<li class={cn("", className)} {...restProps}>
	{@render children?.()}
</li>