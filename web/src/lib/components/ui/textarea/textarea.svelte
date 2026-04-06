<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    value?: string;
    class?: string;
    [key: string]: unknown;
  }

  let { value = $bindable(''), class: className = '', ...restProps }: Props = $props();
</script>

<textarea
  bind:value
  class={cn(
    'flex min-h-24 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
    className
  )}
  {...restProps}
></textarea>
