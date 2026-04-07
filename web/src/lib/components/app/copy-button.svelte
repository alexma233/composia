<script lang="ts">
  import { Copy, Check } from 'lucide-svelte';
  import { Button } from '$lib/components/ui/button';
  import * as Tooltip from '$lib/components/ui/tooltip';

  let { text, label = 'Copy to clipboard' }: { text: string; label?: string } = $props();

  let copied = $state(false);

  function handleCopy() {
    navigator.clipboard.writeText(text);
    copied = true;
    setTimeout(() => (copied = false), 2000);
  }
</script>

<Tooltip.Root>
  <Tooltip.Trigger>
    <Button
      variant="ghost"
      size="icon"
      onclick={handleCopy}
      class="h-6 w-6 text-muted-foreground hover:text-foreground"
    >
      {#if copied}
        <Check class="h-3.5 w-3.5 text-green-500" />
      {:else}
        <Copy class="h-3.5 w-3.5" />
      {/if}
      <span class="sr-only">{label}</span>
    </Button>
  </Tooltip.Trigger>
  <Tooltip.Content>
    <p>{copied ? 'Copied!' : label}</p>
  </Tooltip.Content>
</Tooltip.Root>
