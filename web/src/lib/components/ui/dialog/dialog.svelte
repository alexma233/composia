<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    visible?: boolean;
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { visible = $bindable(false), class: className = '', children, ...restProps }: Props = $props();
</script>

{#if visible}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 bg-black/80" onclick={() => (visible = false)}></div>
  <div class={cn('fixed left-[50%] top-[50%] z-50 grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 border border-border bg-background p-6 shadow-lg duration-200 sm:rounded-lg', className)} {...restProps}>
    {@render children?.()}
  </div>
{/if}