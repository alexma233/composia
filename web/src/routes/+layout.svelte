<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import type { Snippet } from 'svelte';

  import type { LayoutData } from './$types';

  import '../app.css';

  import type { Dictionary } from '$lib/i18n/messages/en-us';
  import { Toaster } from '$lib/components/ui/sonner';
  import { TooltipProvider } from '$lib/components/ui/tooltip';
  import { messages } from '$lib/i18n';
  import { initializePreferences } from '$lib/preferences';
  import { cn } from '$lib/utils';
  import { Select } from '$lib/components/ui/select';
  import SelectContent from '$lib/components/ui/select/select-content.svelte';
  import SelectItem from '$lib/components/ui/select/select-item.svelte';
  import SelectTrigger from '$lib/components/ui/select/select-trigger.svelte';

  type NavKey = keyof Dictionary['nav'];

  interface Props {
    data: LayoutData;
    children?: Snippet;
  }

  let { data, children }: Props = $props();

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

  let selectedService = $state(currentServiceName($page.url.pathname));

  function handleServiceSwitch(value: string) {
    if (value) {
      window.location.href = `/services/${value}`;
    }
  }
</script>

<div class="min-h-screen bg-transparent text-foreground">
  <Toaster />
  <TooltipProvider />
  <header class="sticky top-0 z-30 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
    <div class="mx-auto max-w-[1600px] px-4 py-3 sm:px-6 lg:px-8">
      <div class="scrollbar-none flex items-center gap-3 overflow-x-auto">
        <div class="min-w-0 shrink-0 max-md:hidden">
          <a href="/" class="text-xl font-semibold tracking-tight text-primary sm:text-2xl">{$messages.app.name}</a>
        </div>

        <nav class="flex shrink-0 gap-2 text-sm whitespace-nowrap">
          {#each links as link}
            <a
              href={link.href}
              class={cn(
                'nav-pill',
                isActive(link.href, $page.url.pathname)
                  ? 'nav-pill-active'
                  : 'nav-pill-inactive'
              )}
            >
              {$messages.nav[link.labelKey]}
            </a>
          {/each}
        </nav>

        <div class="ml-auto flex min-w-0 shrink-0 items-center gap-3">
          {#if isServiceWorkspace($page.url.pathname) && data.navServices.length}
            <div class="toolbar-surface flex items-center gap-3 text-sm text-muted-foreground">
              <span class="text-xs font-medium text-muted-foreground">
                {$messages.nav.services}
              </span>
              <Select type="single" bind:value={selectedService as any} onValueChange={(value: string) => handleServiceSwitch(value)}>
                <SelectTrigger class="min-w-36 border-0 bg-transparent p-0 text-sm font-medium text-foreground shadow-none outline-none focus:ring-0">
                  <span class="truncate">
                    {data.navServices.find(s => s.folder === selectedService)?.displayName ?? 'Select...'}
                  </span>
                </SelectTrigger>
                <SelectContent>
                  {#each data.navServices as service}
                    <SelectItem value={service.folder}>{service.displayName}</SelectItem>
                  {/each}
                </SelectContent>
              </Select>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </header>

  {@render children?.()}
</div>
