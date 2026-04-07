<script lang="ts">
  import { setContext } from 'svelte';

  import { tabsContextKey, type TabsContext } from './context';
  import { cn } from '$lib/utils';

  interface Props {
    value?: string;
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { value = '', class: className = '', children, ...restProps }: Props = $props();

  let currentValue = $state('');

  $effect(() => {
    currentValue = value;
  });

  setContext<TabsContext>(tabsContextKey, {
    get value() { return currentValue; },
    setValue: (v: string) => { currentValue = v; }
  });
</script>

<div class={cn('space-y-4', className)} {...restProps}>
  {@render children?.()}
</div>
