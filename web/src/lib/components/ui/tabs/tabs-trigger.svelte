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

<button
  type="button"
  class={cn(
    'inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
    active && 'bg-background text-foreground shadow-xs',
    className
  )}
  data-state={active ? 'active' : 'inactive'}
  onclick={() => context.setValue(value)}
  {...restProps}
>
  {@render children?.()}
</button>
