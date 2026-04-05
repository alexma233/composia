<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';

  import type { LayoutData } from './$types';

  import '../app.css';

  import type { Dictionary } from '$lib/i18n/messages/en-us';
  import { Toaster } from '$lib/components/ui/sonner';
  import { messages } from '$lib/i18n';
  import { initializePreferences } from '$lib/preferences';
  import { cn } from '$lib/utils';

  type NavKey = keyof Dictionary['nav'];

  export let data: LayoutData;

  const links: Array<{ href: string; labelKey: NavKey }> = [
    { href: '/', labelKey: 'overview' },
    { href: '/services', labelKey: 'services' },
    { href: '/nodes', labelKey: 'nodes' },
    { href: '/tasks', labelKey: 'tasks' },
    { href: '/backups', labelKey: 'backups' },
    { href: '/settings', labelKey: 'settings' }
  ];

  onMount(() => initializePreferences());

  function isActive(href: string, pathname: string) {
    return href === '/' ? pathname === '/' : pathname.startsWith(href);
  }

  function isServiceWorkspace(pathname: string) {
    return pathname.startsWith('/services/') && pathname !== '/services';
  }

  function currentServiceName(pathname: string) {
    return isServiceWorkspace(pathname) ? pathname.split('/')[2] ?? '' : '';
  }

  function handleServiceSwitch(event: Event) {
    const target = event.currentTarget as HTMLSelectElement;
    if (target.value) {
      window.location.href = `/services/${target.value}`;
    }
  }
</script>

<div class="min-h-screen bg-transparent text-foreground">
  <Toaster />
  <header class="sticky top-0 z-30 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
    <div class="mx-auto flex max-w-[1600px] flex-col gap-4 px-4 py-3 sm:px-6 lg:px-8">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex min-w-0 flex-wrap items-center gap-3">
          <div class="min-w-0">
            <a href="/" class="text-sm font-semibold text-primary">{$messages.app.name}</a>
          </div>

          {#if isServiceWorkspace($page.url.pathname) && data.navServices.length}
            <label class="flex items-center gap-3 rounded-md border border-border/70 bg-card/80 px-3 py-2 text-sm text-muted-foreground shadow-xs">
              <span class="text-xs font-medium text-muted-foreground">
                Service
              </span>
              <select
                class="min-w-36 bg-transparent text-sm font-medium text-foreground outline-none"
                on:change={handleServiceSwitch}
              >
                {#each data.navServices as service}
                  <option value={service.folder} selected={service.folder === currentServiceName($page.url.pathname)}>{service.displayName}</option>
                {/each}
              </select>
            </label>
          {/if}
        </div>

        <div class="flex flex-col gap-3 lg:items-end">
          <nav class="flex flex-wrap gap-2 text-sm">
            {#each links as link}
              <a
                href={link.href}
                class={cn(
                  'inline-flex h-8 items-center justify-center rounded-md border px-3 text-xs font-medium transition-colors sm:h-9 sm:text-sm',
                  isActive(link.href, $page.url.pathname)
                    ? 'border-border/70 bg-secondary/80 text-secondary-foreground shadow-xs'
                    : 'text-muted-foreground hover:bg-accent/80 hover:text-accent-foreground'
                )}
              >
                {$messages.nav[link.labelKey]}
              </a>
            {/each}
          </nav>
        </div>
      </div>
    </div>
  </header>

  <slot />
</div>
