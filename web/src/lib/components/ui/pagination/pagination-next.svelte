<script lang="ts">
	import type { ComponentProps } from "svelte";
	import { cn } from "$lib/utils.js";
	import { PaginationLink } from "./index.js";
	import { getMessages } from "$lib/i18n";

  const messages = getMessages();
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	type PaginationNextProps = ComponentProps<typeof PaginationLink>;

	let { class: className, ...restProps }: PaginationNextProps = $props();
</script>

<PaginationLink
	aria-label={$messages.common.goToNextPage}
	size="default"
	class={cn("pr-1.5!", className)}
	{...restProps}
>
	<span class="cn-pagination-next-text hidden sm:block">{$messages.common.next}</span>
	<ChevronRightIcon data-icon="inline-end" />
</PaginationLink>
