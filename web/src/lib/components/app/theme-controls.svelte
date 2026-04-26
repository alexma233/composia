<script lang="ts">
  import { Monitor, Moon, Sun } from 'lucide-svelte';

  import { availableLocales, messages, type Locale } from '$lib/i18n';
  import {
    accentColor,
    accentMetadata,
    availableAccentColors,
    preferredLocale,
    setAccentColor,
    setPreferredLocale,
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

  const localeLabels: Record<string, string> = {
    'en-US': 'English',
    'zh-Hans': '简体中文'
  };

  const accentLabelsZhHans: Record<AccentColor, string> = {
    blue: '蓝色',
    emerald: '翠绿',
    violet: '紫罗兰',
    rose: '玫瑰',
    amber: '琥珀'
  };

  function accentLabel(accent: AccentColor) {
    const locale = $preferredLocale as string;
    return locale === 'zh-Hans'
      ? accentLabelsZhHans[accent]
      : accentMetadata[accent].label;
  }
</script>

<div class="space-y-4">
  <div class="space-y-2">
    <div class="text-sm font-medium text-foreground">{$messages.preferences.theme}</div>
    <div class="toolbar-surface flex flex-wrap items-center gap-2">
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
    <div class="toolbar-surface flex flex-wrap items-center gap-2">
      {#each availableAccentColors as accent}
        <Button
          variant={$accentColor === accent ? 'secondary' : 'outline'}
          size="sm"
          class="gap-2"
          aria-label={accentLabel(accent)}
          aria-pressed={$accentColor === accent}
          onclick={() => setAccentColor(accent as AccentColor)}
        >
          <span
            class="size-4 rounded-full border border-black/10 shadow-xs dark:border-white/10"
            style={`background:${accentMetadata[accent].preview}`}
          ></span>
          {accentLabel(accent)}
        </Button>
      {/each}
    </div>
  </div>

  <div class="space-y-2">
    <div class="text-sm font-medium text-foreground">{$messages.preferences.locale}</div>
    <div class="toolbar-surface flex flex-wrap items-center gap-2">
      {#each availableLocales as locale}
        <Button
          variant={$preferredLocale === locale ? 'secondary' : 'ghost'}
          size="sm"
          class="min-w-24 justify-start"
          aria-label={localeLabels[locale]}
          onclick={() => setPreferredLocale(locale)}
        >
          {localeLabels[locale]}
        </Button>
      {/each}
    </div>
  </div>
</div>
