<script lang="ts">
  import { Copy, Check } from '@lucide/svelte';
  import { Button } from '$lib/components/ui/button';
  import * as Tooltip from '$lib/components/ui/tooltip';
  import { getMessages } from '$lib/i18n';

  const messages = getMessages();
  interface Props {
    text: string;
    label?: string;
  }

  let { text, label = $messages.common.copy }: Props = $props();



  let copied = $state(false);

  function handleCopy() {
    navigator.clipboard.writeText(text);
    copied = true;
    setTimeout(() => (copied = false), 2000);
  }
</script>

<Tooltip.Root>
  <Tooltip.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="icon"
        onclick={handleCopy}
        aria-label={copied ? $messages.common.copied : label}
        class="h-6 w-6 text-muted-foreground hover:text-foreground"
      >
        {#if copied}
          <Check class="h-3.5 w-3.5 text-success-foreground" aria-hidden="true" />
        {:else}
          <Copy class="h-3.5 w-3.5" aria-hidden="true" />
        {/if}
      </Button>
    {/snippet}
  </Tooltip.Trigger>
  <Tooltip.Content>
    <p>{copied ? $messages.common.copied : label}</p>
  </Tooltip.Content>
</Tooltip.Root>
