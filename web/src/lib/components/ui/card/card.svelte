<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<div class={cn('rounded-lg border border-border bg-card text-card-foreground shadow-xs', className)} {...$$restProps}>
  <slot />
</div>
