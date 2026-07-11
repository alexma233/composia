<script lang="ts">
  import { afterNavigate } from "$app/navigation";
  import { page } from "$app/stores";
  import { onMount } from "svelte";
  import type { Snippet } from "svelte";

  import type { LayoutData } from "./$types";

  import "../app.css";

  import type { Dictionary } from "$lib/i18n/messages/en-us";
  import { Button } from "$lib/components/ui/button";
  import * as Sheet from "$lib/components/ui/sheet";
  import { Toaster } from "$lib/components/ui/sonner";
  import { TooltipProvider } from "$lib/components/ui/tooltip";
  import { messages } from "$lib/i18n";
  import { initializePreferences } from "$lib/preferences";
  import { cn } from "$lib/utils";
  import { Menu } from "@lucide/svelte";

  type NavKey = keyof Dictionary["nav"];

  interface Props {
    data: LayoutData;
    children?: Snippet;
  }

  type LayoutUser = {
    name: string;
  };

  let { data, children }: Props = $props();
  let pathname = $derived(String($page.url.pathname));
  let isLoginPage = $derived(pathname === "/login");
  let currentUser = $derived(
    ((data as LayoutData & { user?: LayoutUser | null }).user ??
      null) as LayoutUser | null,
  );
  let backupNavEnabled = $derived(
    data.capabilities?.global.backup.enabled !== false,
  );
  let mobileNavOpen = $state(false);
  let appReady = $state(false);

  const links: Array<{ href: string; labelKey: NavKey }> = [
    { href: "/", labelKey: "overview" },
    { href: "/services", labelKey: "services" },
    { href: "/nodes", labelKey: "nodes" },
    { href: "/tasks", labelKey: "tasks" },
    { href: "/settings", labelKey: "settings" },
  ];
  let visibleLinks = $derived(
    backupNavEnabled
      ? [
          ...links.slice(0, 4),
          { href: "/backups", labelKey: "backups" as NavKey },
          links[4],
        ]
      : links,
  );

  onMount(() => {
    appReady = true;
    return initializePreferences();
  });
  afterNavigate(() => (mobileNavOpen = false));

  function isActive(href: string, pathname: string) {
    return href === "/" ? pathname === "/" : pathname.startsWith(href);
  }
</script>

<div
  class="min-h-screen bg-transparent text-foreground"
  data-app-ready={appReady ? "" : undefined}
>
  <Toaster />
  <TooltipProvider />
  {#if !isLoginPage}
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground focus:no-underline"
    >
      {$messages.common.skipToContent}
    </a>
    <header
      class="sticky top-0 z-30 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80"
    >
      <div class="mx-auto max-w-[1600px] px-4 py-3 sm:px-6 lg:px-8">
        <div class="flex items-center gap-3">
          <a
            href="/"
            class="flex min-w-0 items-center gap-2 text-xl font-semibold tracking-tight text-primary sm:text-2xl"
          >
            <img
              src="/favicon.svg"
              alt=""
              width="28"
              height="28"
              class="shrink-0"
            />
            <span class="truncate">{$messages.app.name}</span>
          </a>

          <nav
            class="ml-2 hidden shrink-0 gap-2 text-sm whitespace-nowrap md:flex"
            aria-label={$messages.nav.navLabel}
          >
            {#each visibleLinks as link}
              <a
                href={link.href}
                class={cn(
                  "nav-pill",
                  isActive(link.href, $page.url.pathname)
                    ? "nav-pill-active"
                    : "nav-pill-inactive",
                )}
              >
                {$messages.nav[link.labelKey]}
              </a>
            {/each}
          </nav>

          <div class="ml-auto hidden min-w-0 shrink-0 items-center gap-3 md:flex">
            {#if currentUser}
              <span class="text-sm text-muted-foreground">{currentUser.name}</span>
              <form method="POST" action="/logout">
                <button type="submit" class="nav-pill nav-pill-inactive"
                  >{$messages.nav.logout}</button
                >
              </form>
            {/if}
          </div>

          <Sheet.Root bind:open={mobileNavOpen}>
            <Sheet.Trigger>
              {#snippet child({ props })}
                <Button
                  {...props}
                  type="button"
                  variant="outline"
                  size="icon"
                  class="ml-auto md:hidden"
                  aria-label={$messages.nav.navLabel}
                >
                  <Menu class="size-4" aria-hidden="true" />
                </Button>
              {/snippet}
            </Sheet.Trigger>
            <Sheet.Content side="right" class="w-[min(22rem,85vw)]">
              <Sheet.Header class="border-b px-5 py-4">
                <Sheet.Title>{$messages.app.name}</Sheet.Title>
              </Sheet.Header>
              <nav
                class="flex flex-1 flex-col gap-2 overflow-y-auto px-4"
                aria-label={$messages.nav.navLabel}
              >
                {#each visibleLinks as link}
                  <a
                    href={link.href}
                    aria-current={isActive(link.href, pathname)
                      ? "page"
                      : undefined}
                    class={cn(
                      "nav-pill h-10 w-full justify-start px-4 text-sm",
                      isActive(link.href, pathname)
                        ? "nav-pill-active"
                        : "nav-pill-inactive",
                    )}
                  >
                    {$messages.nav[link.labelKey]}
                  </a>
                {/each}
              </nav>
              {#if currentUser}
                <Sheet.Footer class="border-t px-4 py-4">
                  <span class="truncate text-sm text-muted-foreground"
                    >{currentUser.name}</span
                  >
                  <form method="POST" action="/logout">
                    <button
                      type="submit"
                      class="nav-pill nav-pill-inactive h-10 w-full justify-start px-4 text-sm"
                    >{$messages.nav.logout}</button>
                  </form>
                </Sheet.Footer>
              {/if}
            </Sheet.Content>
          </Sheet.Root>
        </div>
      </div>
    </header>
  {/if}

  <main id="main-content">
    {@render children?.()}
  </main>
</div>
