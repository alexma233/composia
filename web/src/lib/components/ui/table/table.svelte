<script lang="ts">
  import { cn } from '$lib/utils';

  export let className = '';

  export { className as class };
</script>

<div class={cn('relative w-full overflow-x-auto', className)}>
  <table class="w-full caption-bottom text-sm" {...$$restProps}>
    <slot />
  </table>
</div>
