<script lang="ts">
  import { getContext } from 'svelte';

  import { cn } from '$lib/utils';
  import { tabsContextKey, type TabsContext } from './context';

  const context = getContext<TabsContext>(tabsContextKey);
  const selected = context.value;

  export let value = '';
  export let className = '';

  $: active = $selected === value;

  export { className as class };
</script>

<button
  type="button"
  class={cn(
    'inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
    active && 'bg-background text-foreground shadow-xs',
    className
  )}
  data-state={active ? 'active' : 'inactive'}
  on:click={() => selected.set(value)}
  {...$$restProps}
>
  <slot />
</button>
