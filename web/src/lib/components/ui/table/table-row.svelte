<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<tr class={cn('border-b transition-colors hover:bg-muted/40 data-[state=selected]:bg-muted', className)} {...$$restProps}>
  <slot />
</tr>
