<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<th class={cn('h-10 px-3 text-left align-middle text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground', className)} {...$$restProps}>
  <slot />
</th>
