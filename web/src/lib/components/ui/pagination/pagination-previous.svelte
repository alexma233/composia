<script lang="ts">
	import type { ComponentProps } from "svelte";
	import { cn } from "$lib/utils.js";
	import { PaginationLink } from "./index.js";
	import { messages } from "$lib/i18n";
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';

	type PaginationPreviousProps = ComponentProps<typeof PaginationLink>;

	let { class: className, ...restProps }: PaginationPreviousProps = $props();
</script>

<PaginationLink
	aria-label={$messages.common.goToPreviousPage}
	size="default"
	class={cn("pl-1.5!", className)}
	{...restProps}
>
	<ChevronLeftIcon data-icon="inline-start" />
	<span class="cn-pagination-previous-text hidden sm:block">{$messages.common.previous}</span>
</PaginationLink>
