<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<div class={cn('text-sm leading-6 text-current [&_p]:leading-6', className)} {...$$restProps}>
  <slot />
</div>
