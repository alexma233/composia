<script lang="ts">
  import type { Snippet } from 'svelte';

  import * as Tooltip from '$lib/components/ui/tooltip';

  interface Props {
    reason?: string;
    triggerClass?: string;
    children?: Snippet;
  }

  let { reason = "", triggerClass = "inline-flex", children }: Props = $props();
</script>

{#if reason}
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        <span class={triggerClass} {...props}>
          {@render children?.()}
        </span>
      {/snippet}
    </Tooltip.Trigger>
    <Tooltip.Content>
      <p>{reason}</p>
    </Tooltip.Content>
  </Tooltip.Root>
{:else}
  {@render children?.()}
{/if}
