<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { class: className = '', children, ...restProps }: Props = $props();
</script>

<div class={cn('relative w-full overflow-x-auto', className)}>
  <table class="w-full caption-bottom text-sm" {...restProps}>
    {@render children?.()}
  </table>
</div>
