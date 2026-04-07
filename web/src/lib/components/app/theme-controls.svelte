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

  const themeOptions: Array<{ value: ThemeMode; icon: typeof Sun; labelKey: 'light' | 'dark' | 'system' }> = [
    { value: 'light', icon: Sun, labelKey: 'light' },
    { value: 'dark', icon: Moon, labelKey: 'dark' },
    { value: 'system', icon: Monitor, labelKey: 'system' }
  ];
</script>

<div class="space-y-4">
  <div class="space-y-2">
    <div class="text-sm font-medium text-foreground">Theme</div>
    <div class="flex flex-wrap items-center gap-2 rounded-lg border border-border/70 bg-background/80 p-2 shadow-xs">
      {#each themeOptions as option}
        <Button
          variant={$themeMode === option.value ? 'secondary' : 'ghost'}
          size="sm"
          class="min-w-24 justify-start"
          aria-label={$messages.preferences[option.labelKey]}
          onclick={() => setThemeMode(option.value)}
        >
          <svelte:component this={option.icon} />
          {$messages.preferences[option.labelKey]}
        </Button>
      {/each}
    </div>
  </div>

  <div class="space-y-2">
    <div class="text-sm font-medium text-foreground">{$messages.preferences.accent}</div>
    <div class="flex flex-wrap items-center gap-2 rounded-lg border border-border/70 bg-background/80 p-2 shadow-xs">
      {#each availableAccentColors as accent}
        <Button
          variant={$accentColor === accent ? 'secondary' : 'outline'}
          size="sm"
          class="gap-2"
          aria-label={accentMetadata[accent].label}
          aria-pressed={$accentColor === accent}
          onclick={() => setAccentColor(accent as AccentColor)}
        >
          <span
            class="size-4 rounded-full border border-black/10 shadow-xs dark:border-white/10"
            style={`background:${accentMetadata[accent].preview}`}
          ></span>
          {accentMetadata[accent].label}
        </Button>
      {/each}
    </div>
  </div>
</div>
