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

        <div class="ml-auto flex min-w-0 shrink-0 items-center gap-3"></div>
      </div>
    </div>
  </header>

  {@render children?.()}
</div>
