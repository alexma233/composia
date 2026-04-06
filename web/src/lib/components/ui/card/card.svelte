<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { class: className = '', children, ...restProps }: Props = $props();
</script>

<div class={cn('rounded-lg border border-border bg-card text-card-foreground shadow-xs', className)} {...restProps}>
  {@render children?.()}
</div>
