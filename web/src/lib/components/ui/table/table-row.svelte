<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { class: className = '', children, ...restProps }: Props = $props();
</script>

<tr class={cn('border-b transition-colors hover:bg-muted/40 data-[state=selected]:bg-muted', className)} {...restProps}>
  {@render children?.()}
</tr>
