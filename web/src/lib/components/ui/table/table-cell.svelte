<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<td class={cn('p-3 align-middle', className)} {...$$restProps}>
  <slot />
</td>
