<script lang="ts">
  import { Toaster as Sonner, type ToasterProps } from 'svelte-sonner';

  import { themeMode } from '$lib/preferences';

  let restProps: ToasterProps = {};
</script>

<Sonner
  theme={$themeMode}
  class="toaster group"
  style="--normal-bg: var(--color-popover); --normal-text: var(--color-popover-foreground); --normal-border: var(--color-border);"
  position="top-right"
  closeButton
  richColors
  {...restProps}
/>
