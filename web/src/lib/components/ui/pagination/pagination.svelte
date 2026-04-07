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

<nav
	aria-label="pagination"
	class={cn("mx-auto flex w-full justify-center", className)}
	{...restProps}
>
	{@render children?.()}
</nav>