<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';

  import type { LayoutData } from './$types';

  import '../app.css';

  import type { Dictionary } from '$lib/i18n/messages/en-us';
  import { messages } from '$lib/i18n';
  import { initializePreferences } from '$lib/preferences';
  import ThemeControls from '$lib/components/app/theme-controls.svelte';
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
  <header class="sticky top-0 z-30 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
    <div class="mx-auto flex max-w-[1600px] flex-col gap-4 px-4 py-3 sm:px-6 lg:px-8">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex min-w-0 flex-wrap items-center gap-3">
          {#if isServiceWorkspace($page.url.pathname)}
            <a href="/services" class="inline-flex h-9 items-center rounded-md border bg-background px-3 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground">
              Back
            </a>
          {/if}

          <div class="min-w-0">
            <a href="/" class="text-sm font-semibold tracking-[0.24em] uppercase text-primary">{$messages.app.name}</a>
            <p class="text-sm text-muted-foreground">{$messages.app.subtitle}</p>
          </div>

          {#if isServiceWorkspace($page.url.pathname) && data.navServices.length}
            <label class="flex items-center gap-2 rounded-md border bg-background px-3 py-2 text-sm text-muted-foreground">
              <span>Service</span>
              <select class="bg-transparent text-foreground outline-none" on:change={handleServiceSwitch}>
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
    </div>
  </header>

  <slot />
</div>
