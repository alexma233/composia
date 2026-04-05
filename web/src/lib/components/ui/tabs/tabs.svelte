<script lang="ts">
  import { setContext } from 'svelte';
  import { writable } from 'svelte/store';

  import { tabsContextKey, type TabsContext } from './context';
  import { cn } from '$lib/utils';

  export let value = '';
  export let className = '';

  const currentValue = writable(value);

  setContext<TabsContext>(tabsContextKey, {
    value: currentValue
  });

  export { className as class };
</script>

<div class={cn('space-y-4', className)} {...$$restProps}>
  <slot />
</div>
