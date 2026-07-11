<script lang="ts">
  import type { ButtonProps } from "$lib/components/ui/button";
  import { Button } from "$lib/components/ui/button";
  import * as Tooltip from "$lib/components/ui/tooltip";
  import { cn } from "$lib/utils";

  type Props = ButtonProps & {
    reason?: string;
    triggerClass?: string;
  };

  let {
    reason = "",
    triggerClass = "",
    class: className,
    onclick,
    children,
    ...restProps
  }: Props = $props();
</script>

{#if reason}
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        <Button
          {...restProps}
          {...props}
          disabled={false}
          aria-disabled="true"
          class={cn("cursor-not-allowed opacity-50", triggerClass, className)}
          onclick={(event) => {
            event.preventDefault();
            event.stopPropagation();
          }}
        >
          {@render children?.()}
        </Button>
      {/snippet}
    </Tooltip.Trigger>
    <Tooltip.Content>
      <p>{reason}</p>
    </Tooltip.Content>
  </Tooltip.Root>
{:else}
  <Button {...restProps} class={cn(triggerClass, className)} {onclick}>
    {@render children?.()}
  </Button>
{/if}
