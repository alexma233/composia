<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<caption class={cn('mt-4 text-sm text-muted-foreground', className)} {...$$restProps}>
  <slot />
</caption>
