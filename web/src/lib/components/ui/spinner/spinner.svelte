<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { messages } from "$lib/i18n";
	import Loader2Icon from '@lucide/svelte/icons/loader-2';
	import type { SVGAttributes } from "svelte/elements";

	let {
		class: className,
		role = "status",
		// we add name, color, and stroke for compatibility with different icon libraries props
		name,
		color,
		stroke,
		"aria-label": ariaLabel = $messages.common.loading,
		...restProps
	}: SVGAttributes<SVGSVGElement> = $props();
</script>

<Loader2Icon {role} name={name === null ? undefined : name} color={color === null ? undefined : color} stroke={stroke === null ? undefined : stroke} aria-label={ariaLabel} class={cn("size-4 animate-spin", className)} {...restProps} />
