<script lang="ts">
	import { cn } from "$lib/utils.js";
	import PopoverPortal from "./popover-portal.svelte";
	import { Popover as PopoverPrimitive } from "bits-ui";
	import type { ComponentProps, Snippet } from "svelte";

	let {
		ref = $bindable(null),
		sideOffset = 4,
		align = "center",
		portalProps,
		class: className,
		children,
		...restProps
	}: PopoverPrimitive.ContentProps & {
		portalProps?: ComponentProps<typeof PopoverPortal>;
		children?: Snippet;
	} = $props();
</script>

<PopoverPortal {...portalProps}>
	<PopoverPrimitive.Content
		bind:ref
		data-slot="popover-content"
		{sideOffset}
		{align}
		class={cn(
			"bg-popover text-popover-foreground data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-95 data-open:zoom-in-95 z-50 w-72 rounded-lg border p-4 shadow-md outline-none ring-1 duration-100",
			className
		)}
		{...restProps}
	>
		{@render children?.()}
	</PopoverPrimitive.Content>
</PopoverPortal>
