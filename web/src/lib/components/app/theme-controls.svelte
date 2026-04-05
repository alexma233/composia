<script lang="ts">
  import { Monitor, Moon, Sun } from 'lucide-svelte';

  import { messages } from '$lib/i18n';
  import {
    accentColor,
    accentMetadata,
    availableAccentColors,
    setAccentColor,
    setThemeMode,
    themeMode,
    type AccentColor,
    type ThemeMode
  } from '$lib/preferences';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';

  const themeOptions: Array<{ value: ThemeMode; icon: typeof Sun; labelKey: 'light' | 'dark' | 'system' }> = [
    { value: 'light', icon: Sun, labelKey: 'light' },
    { value: 'dark', icon: Moon, labelKey: 'dark' },
    { value: 'system', icon: Monitor, labelKey: 'system' }
  ];
</script>

<div class="flex flex-wrap items-center gap-3">
  <div class="flex items-center gap-1 rounded-md border border-border bg-muted/40 p-1">
    {#each themeOptions as option}
      <Button
        variant={$themeMode === option.value ? 'secondary' : 'ghost'}
        size="sm"
        aria-label={$messages.preferences[option.labelKey]}
        on:click={() => setThemeMode(option.value)}
      >
        <svelte:component this={option.icon} />
      </Button>
    {/each}
  </div>

  <div class="hidden items-center gap-2 md:flex">
    <Badge variant="outline">{$messages.preferences.accent}</Badge>
    <div class="flex items-center gap-1">
      {#each availableAccentColors as accent}
        <button
          type="button"
          class="size-7 rounded-full border transition-transform hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          class:border-foreground={$accentColor === accent}
          class:border-border={$accentColor !== accent}
          style={`background:${accentMetadata[accent].preview}`}
          aria-label={accentMetadata[accent].label}
          aria-pressed={$accentColor === accent}
          on:click={() => setAccentColor(accent as AccentColor)}
        ></button>
      {/each}
    </div>
  </div>
</div>
