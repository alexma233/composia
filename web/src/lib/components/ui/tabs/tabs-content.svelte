<script lang="ts">
  import { getContext } from 'svelte';

  import { cn } from '$lib/utils';
  import { tabsContextKey, type TabsContext } from './context';

  interface Props {
    value?: string;
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { value = '', class: className = '', children, ...restProps }: Props = $props();

  const context = getContext<TabsContext>(tabsContextKey);
  let active = $derived(context.value === value);
</script>

{#if active}
  <div class={cn('focus-visible:outline-none', className)} data-state="active" {...restProps}>
    {@render children?.()}
  </div>
{/if}
