<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';

  import '../app.css';

  import type { Dictionary } from '$lib/i18n/messages/en-us';
  import { messages } from '$lib/i18n';
  import { initializePreferences } from '$lib/preferences';
  import ThemeControls from '$lib/components/app/theme-controls.svelte';
  import { cn } from '$lib/utils';

  type NavKey = keyof Dictionary['nav'];

  const links: Array<{ href: string; labelKey: NavKey }> = [
    { href: '/', labelKey: 'overview' },
    { href: '/services', labelKey: 'services' },
    { href: '/nodes', labelKey: 'nodes' },
    { href: '/tasks', labelKey: 'tasks' },
    { href: '/backups', labelKey: 'backups' },
    { href: '/repo', labelKey: 'repo' }
  ];

  onMount(() => initializePreferences());

  function isActive(href: string, pathname: string) {
    return href === '/' ? pathname === '/' : pathname.startsWith(href);
  }
</script>

<div class="min-h-screen bg-transparent text-foreground">
  <header class="sticky top-0 z-30 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
    <div class="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8">
      <div class="min-w-0">
        <a href="/" class="text-sm font-semibold tracking-[0.24em] uppercase text-primary">{$messages.app.name}</a>
        <p class="text-sm text-muted-foreground">{$messages.app.subtitle}</p>
      </div>

      <div class="flex flex-col gap-3 lg:items-end">
        <nav class="flex flex-wrap gap-2 text-sm">
        {#each links as link}
          <a
            href={link.href}
            class={cn(
              'inline-flex h-9 items-center rounded-md border px-3 text-sm transition-colors',
              isActive(link.href, $page.url.pathname)
                ? 'border-border bg-secondary text-secondary-foreground'
                : 'border-transparent text-muted-foreground hover:bg-accent hover:text-accent-foreground'
            )}
          >
            {$messages.nav[link.labelKey]}
          </a>
        {/each}
        </nav>

        <ThemeControls />
      </div>
    </div>
  </header>

  <slot />
</div>
