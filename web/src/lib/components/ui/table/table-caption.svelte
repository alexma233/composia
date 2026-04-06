<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { class: className = '', children, ...restProps }: Props = $props();
</script>

<caption class={cn('mt-4 text-sm text-muted-foreground', className)} {...restProps}>
  {@render children?.()}
</caption>
