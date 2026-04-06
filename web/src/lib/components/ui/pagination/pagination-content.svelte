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

<ul
	class={cn("flex flex-row items-center gap-1", className)}
	{...restProps}
>
	{@render children?.()}
</ul>