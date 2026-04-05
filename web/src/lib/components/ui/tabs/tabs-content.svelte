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

{#if active}
  <div class={cn('focus-visible:outline-none', className)} data-state="active" {...$$restProps}>
    <slot />
  </div>
{/if}
