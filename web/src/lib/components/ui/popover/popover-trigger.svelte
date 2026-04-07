<script lang="ts">
	import { Popover as PopoverPrimitive } from "bits-ui";
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		child,
		children,
		...restProps
	}: PopoverPrimitive.TriggerProps & {
		child?: Snippet<[{ props: Record<string, unknown> }]>;
	} = $props();
</script>

<PopoverPrimitive.Trigger
	bind:ref
	data-slot="popover-trigger"
	class={cn("inline-flex items-center justify-center", className)}
	{child}
	{...restProps}
>
	{@render children?.()}
</PopoverPrimitive.Trigger>
