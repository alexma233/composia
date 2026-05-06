<script lang="ts">
	import type { HTMLAttributes } from "svelte/elements";
	import { cn, type WithElementRef } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		children,
		level = undefined as "1" | "2" | "3" | "4" | "5" | "6" | undefined,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLHeadingElement>> & {
		level?: "1" | "2" | "3" | "4" | "5" | "6";
	} = $props();

	let tag = $derived(level ? `h${level}` : "div");
</script>

<svelte:element
	this={tag}
	bind:this={ref}
	data-slot="card-title"
	class={cn("text-base leading-snug font-medium group-data-[size=sm]/card:text-sm", className)}
	{...restProps}
>
	{@render children?.()}
</svelte:element>
