<script lang="ts">
	import { cn } from "$lib/utils";
	import PaginationItem from "./pagination-item.svelte";

	let {
		class: className,
		...restProps
	}: { class?: string } = $props();
</script>

<PaginationItem>
	<span class={cn("flex h-9 w-9 items-center justify-center", className)} {...restProps}>
		&#8230;
	</span>
</PaginationItem>