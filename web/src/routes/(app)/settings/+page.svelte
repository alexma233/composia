<script lang="ts">
  import { invalidate } from "$app/navigation";
  import { RefreshCw } from "@lucide/svelte";
  import { onMount } from "svelte";
  import { toast } from "svelte-sonner";

  import type { PageData } from "./$types";
  import type { RepoCommitSummary } from "$lib/server/controller";
  import { actionErrorMessage, globalCapability } from "$lib/capabilities";
  import { getMessages } from "$lib/i18n";

  const messages = getMessages();

  import { startPolling } from "$lib/refresh";
  import ThemeControls from "$lib/components/app/theme-controls.svelte";
  import {
    Alert,
    AlertDescription,
    AlertTitle,
  } from "$lib/components/ui/alert";
  import { Button } from "$lib/components/ui/button";
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationLink,
    PaginationPrevButton,
    PaginationNextButton,
  } from "$lib/components/ui/pagination";
  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let syncing = $state(false);
  let syncError = $state("");
  let reloadingController = $state(false);
  let reloadControllerError = $state("");
  let reloadAccepted = $state<boolean | null>(null);
  let rusticBusy = $state<"init" | "forget" | "prune" | "">("");
  let rusticError = $state("");
  let rusticTaskId = $state("");
  let syncResult = $state<{
    headRevision?: string;
    syncStatus?: string;
    lastSyncError?: string;
    lastSuccessfulPullAt?: string;
  } | null>(null);

  type CommitPage = {
    commits: RepoCommitSummary[];
    nextCursor: string;
  };

  const getInitialCommits = () => data.initialCommits;
  let perPage = 10;
  let pages = $state<CommitPage[]>([
    {
      commits: getInitialCommits()?.commits ?? [],
      nextCursor: getInitialCommits()?.nextCursor ?? "",
    },
  ]);
  let currentPage = $state(1);
  let loadingPage = $state(false);
  let pageError = $state("");

  let hasMore = $derived(!!pages[pages.length - 1]?.nextCursor);
  let count = $derived(Math.max(perPage, (pages.length + (hasMore ? 1 : 0)) * perPage));
  let currentCommits = $derived(pages[currentPage - 1]?.commits ?? []);
  let hasCommits = $derived(currentCommits.length > 0 || pages[0].commits.length > 0);

  $effect(() => {
    const targetIndex = currentPage - 1;
    if (targetIndex < pages.length || targetIndex !== pages.length) return;
    fetchNextPage();
  });

  async function fetchNextPage() {
    const lastPage = pages[pages.length - 1];
    if (!lastPage?.nextCursor || loadingPage) return;

    loadingPage = true;
    pageError = "";

    try {
      const params = new URLSearchParams({
        pageSize: String(perPage),
        cursor: lastPage.nextCursor,
      });
      const response = await fetch(`/settings/commits?${params}`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(
          actionErrorMessage(payload, $messages, $messages.settings.repoSync.commitFailed),
        );
      }

      pages = [...pages, { commits: payload.commits ?? [], nextCursor: payload.nextCursor ?? "" }];
    } catch (error) {
      pageError =
        error instanceof Error
          ? error.message
          : $messages.settings.repoSync.commitFailed;
      currentPage = pages.length;
    } finally {
      loadingPage = false;
    }
  }

  async function syncRepo() {
    syncing = true;
    syncError = "";
    syncResult = null;

    try {
      const response = await fetch("/settings/sync", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });

      const payload = await response.json();
      if (!response.ok) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.error.syncFailed));
      }

      syncResult = {
        headRevision: payload.headRevision,
        syncStatus: payload.syncStatus,
        lastSyncError: payload.lastSyncError,
        lastSuccessfulPullAt: payload.lastSuccessfulPullAt,
      };
      toast.success($messages.settings.repoSync.syncedSuccessfully);
    } catch (error) {
      syncError =
        error instanceof Error ? error.message : $messages.error.syncFailed;
    } finally {
      syncing = false;
    }
  }

  async function reloadControllerConfig() {
    reloadingController = true;
    reloadControllerError = "";
    reloadAccepted = null;

    try {
      const response = await fetch("/settings/reload", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });

      const payload = (await response.json()) as {
        accepted?: boolean;
        error?: string;
      };
      if (!response.ok) {
        throw new Error(
          actionErrorMessage(
            payload,
            $messages,
            $messages.settings.controller.reloadFailed,
          ),
        );
      }

      reloadAccepted = payload.accepted ?? false;
      toast.success($messages.settings.controller.reloadAccepted);
      await Promise.all([
        invalidate("app:settings"),
        invalidate("app:capabilities"),
      ]);
    } catch (error) {
      reloadControllerError =
        error instanceof Error
          ? error.message
          : $messages.settings.controller.reloadFailed;
    } finally {
      reloadingController = false;
    }
  }

  async function runRusticAction(action: "init" | "forget" | "prune") {
    rusticBusy = action;
    rusticError = "";
    rusticTaskId = "";

    try {
      const response = await fetch(`/settings/${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });

      const payload = (await response.json()) as {
        taskId?: string;
        error?: string;
        reasonCode?: string;
      };
      if (!response.ok) {
        throw new Error(
          actionErrorMessage(
            payload,
            $messages,
            $messages.settings.rustic.failedToStart.replace("{action}", action),
          ),
        );
      }

      rusticTaskId = payload.taskId ?? "";
      toast.success(
        $messages.settings.rustic.started.replace("{action}", action),
      );
    } catch (error) {
      rusticError =
        error instanceof Error
          ? error.message
          : $messages.settings.rustic.failedToStart.replace("{action}", action);
    } finally {
      rusticBusy = "";
    }
  }

  let displayHeadRevision = $derived(
    syncResult?.headRevision ?? data.repoHead?.headRevision ?? "",
  );
  let displaySyncStatus = $derived(
    syncResult?.syncStatus ??
      data.repoHead?.syncStatus ??
      $messages.common.unknown,
  );
  let displayLastSyncError = $derived(
    syncResult?.lastSyncError ?? data.repoHead?.lastSyncError ?? "",
  );
  let displayLastPull = $derived(
    syncResult?.lastSuccessfulPullAt ??
      data.repoHead?.lastSuccessfulPullAt ??
      $messages.common.never,
  );
  let rusticMaintenanceCapability = $derived(
    globalCapability(data.capabilities?.global, "rusticMaintenance"),
  );

  onMount(() => startPolling(() => invalidate("app:settings"), { intervalMs: 5000 }));
</script>

<svelte:head>
  <title>{$messages.settings.title} - {$messages.app.name}</title>
</svelte:head>

<div
  class="page-shell"
  aria-busy={syncing || reloadingController || Boolean(rusticBusy) || loadingPage}
>
  <Card class="mb-6">
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title" level="1">{$messages.settings.title}</CardTitle>
        </div>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>
  </Card>

  <div class="page-stack">
    <section class="grid gap-6 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle class="section-title" level="2">{$messages.settings.appearance.title}</CardTitle>
        </CardHeader>
        <CardContent>
          <ThemeControls />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="section-title" level="2">{$messages.settings.actions.title}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={reloadControllerConfig}
              disabled={reloadingController}
            >
              <RefreshCw class="mr-2 size-4" />
              {reloadingController
                ? $messages.settings.controller.reloading
                : $messages.settings.controller.reloadConfig}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={syncRepo}
              disabled={syncing}
            >
              <RefreshCw class="mr-2 size-4" />
              {syncing
                ? $messages.settings.repoSync.syncing
                : $messages.settings.repoSync.syncRepo}
            </Button>
          </div>

          {#if rusticMaintenanceCapability.enabled}
            <div class="space-y-2">
              <CardTitle class="section-label" level="3">{$messages.settings.rustic.title}</CardTitle>
              <div class="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onclick={() => runRusticAction("init")}
                  disabled={rusticBusy !== ""}
                >
                  {rusticBusy === "init"
                    ? $messages.settings.rustic.starting
                    : $messages.settings.rustic.init}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onclick={() => runRusticAction("forget")}
                  disabled={rusticBusy !== ""}
                >
                  {rusticBusy === "forget"
                    ? $messages.settings.rustic.starting
                    : $messages.settings.rustic.forget}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onclick={() => runRusticAction("prune")}
                  disabled={rusticBusy !== ""}
                >
                  {rusticBusy === "prune"
                    ? $messages.settings.rustic.starting
                    : $messages.settings.rustic.prune}
                </Button>
              </div>
            </div>
          {/if}

          {#if reloadControllerError}
            <Alert variant="destructive">
              <AlertTitle>{$messages.settings.controller.reloadFailed}</AlertTitle>
              <AlertDescription>{reloadControllerError}</AlertDescription>
            </Alert>
          {/if}

          {#if reloadAccepted === true}
            <Alert>
              <AlertTitle>{$messages.common.success}</AlertTitle>
              <AlertDescription>{$messages.settings.controller.reloadAccepted}</AlertDescription>
            </Alert>
          {/if}

          {#if syncError}
            <Alert variant="destructive">
              <AlertTitle>{$messages.error.syncFailed}</AlertTitle>
              <AlertDescription>{syncError}</AlertDescription>
            </Alert>
          {/if}

          {#if rusticError}
            <Alert variant="destructive">
              <AlertTitle>{$messages.error.taskError}</AlertTitle>
              <AlertDescription>{rusticError}</AlertDescription>
            </Alert>
          {/if}

          {#if rusticTaskId}
            <div class="inset-card">
              <div class="metric-label">
                {$messages.settings.rustic.lastTask}
              </div>
              <div class="mt-2 break-all text-sm text-foreground">
                {rusticTaskId}
              </div>
            </div>
          {/if}
        </CardContent>
      </Card>

      <Card class="lg:col-span-2">
        <CardHeader>
          <CardTitle class="section-title" level="2">{$messages.settings.status.title}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-6">
          <section>
            <CardTitle class="section-label" level="3">{$messages.settings.controller.title}</CardTitle>
            {#if data.system}
              <dl class="mt-3 grid gap-4 sm:grid-cols-2">
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.controller.version}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.system.version}
                  </dd>
                </div>
              </dl>
            {:else}
              <div class="empty-state mt-3">
                {$messages.settings.controller.noData}
              </div>
            {/if}
          </section>

          <section class="border-t border-border pt-6">
            <CardTitle class="section-label" level="3">{$messages.settings.repoSync.title}</CardTitle>
            {#if data.repoHead || syncResult}
              <dl class="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.repoSync.branch}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.repoHead?.branch || $messages.settings.repoSync.head}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.repoSync.syncStatus}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {displaySyncStatus}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.repoSync.worktree}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.repoHead?.cleanWorktree
                      ? $messages.status.clean
                      : $messages.status.dirty}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.repoSync.lastPull}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {displayLastPull}
                  </dd>
                </div>
              </dl>

              <div class="mt-4 inset-card">
                <div class="metric-label">
                  {$messages.settings.repoSync.revision}
                </div>
                <div class="mt-2 break-all text-sm text-foreground">
                  {displayHeadRevision}
                </div>
              </div>

              {#if displayLastSyncError}
                <Alert variant="destructive">
                  <AlertTitle>{$messages.error.lastSyncError}</AlertTitle>
                  <AlertDescription>{displayLastSyncError}</AlertDescription>
                </Alert>
              {/if}
            {:else}
              <div class="empty-state mt-3">
                {$messages.settings.repoSync.noRepoState}
              </div>
            {/if}
          </section>

          {#if data.currentConfig?.git}
            <section class="border-t border-border pt-6">
              <CardTitle class="section-label" level="3">{$messages.settings.gitConfig.title}</CardTitle>
              <dl class="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.gitConfig.remoteUrl}
                  </dt>
                  <dd class="mt-2 break-all text-sm font-medium text-foreground">
                    {data.currentConfig.git.remoteUrl}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.gitConfig.branch}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.currentConfig.git.branch}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.gitConfig.pullInterval}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.currentConfig.git.pullInterval}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.gitConfig.hasAuth}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.currentConfig.git.hasAuth
                      ? $messages.settings.gitConfig.yes
                      : $messages.settings.gitConfig.no}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.gitConfig.authorName}
                  </dt>
                  <dd class="mt-2 text-sm font-medium text-foreground">
                    {data.currentConfig.git.authorName || "—"}
                  </dd>
                </div>
                <div class="metric-card">
                  <dt class="metric-label">
                    {$messages.settings.gitConfig.authorEmail}
                  </dt>
                  <dd class="mt-2 break-all text-sm font-medium text-foreground">
                    {data.currentConfig.git.authorEmail || "—"}
                  </dd>
                </div>
              </dl>
            </section>
          {/if}
        </CardContent>
      </Card>

      {#if hasCommits}
        <Card>
          <CardHeader class="section-header">
            <div class="section-heading">
              <CardTitle class="section-title" level="2"
                >{$messages.settings.repoSync.commitHistory}</CardTitle
              >
            </div>
          </CardHeader>
          <CardContent class="space-y-4">
            {#if pageError}
              <Alert variant="destructive">
                <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
                <AlertDescription>{pageError}</AlertDescription>
              </Alert>
            {/if}

            {#if currentCommits.length > 0}
              <div class="space-y-3">
                {#each currentCommits as commit}
                  <div class="border-b border-border pb-3 last:border-b-0 last:pb-0">
                    <div class="flex items-start justify-between gap-2">
                      <code class="text-sm font-medium break-all">{commit.commitId}</code>
                      <span class="shrink-0 text-xs text-muted-foreground">{commit.committedAt}</span>
                    </div>
                    <div class="mt-1 text-sm text-foreground">{commit.subject}</div>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="empty-state">
                {$messages.settings.repoSync.noCommits}
              </div>
            {/if}

            {#if pages.length > 1 || hasMore}
              <Pagination {count} {perPage} bind:page={currentPage}>
                <PaginationContent>
                  <PaginationItem>
                    <PaginationPrevButton disabled={loadingPage} />
                  </PaginationItem>
                  {#each pages as _, i}
                    {@const pageNum = i + 1}
                    <PaginationItem>
                      <PaginationLink
                        page={{ value: pageNum, type: "page" }}
                        isActive={pageNum === currentPage}
                      >
                        {pageNum}
                      </PaginationLink>
                    </PaginationItem>
                  {/each}
                  <PaginationItem>
                    <PaginationNextButton disabled={!hasMore || loadingPage} />
                  </PaginationItem>
                </PaginationContent>
              </Pagination>
            {/if}
          </CardContent>
        </Card>
      {/if}

      {#if data.currentConfig?.accessTokens && data.currentConfig.accessTokens.length > 0}
        <Card>
          <CardHeader class="section-header">
            <div class="section-heading">
              <CardTitle class="section-title" level="2"
                >{$messages.settings.accessTokens.title}</CardTitle
              >
            </div>
          </CardHeader>
          <CardContent>
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-border">
                    <th class="px-3 py-2 text-left font-medium text-muted-foreground">
                      {$messages.settings.accessTokens.name}
                    </th>
                    <th class="px-3 py-2 text-left font-medium text-muted-foreground">
                      {$messages.settings.accessTokens.enabled}
                    </th>
                    <th class="px-3 py-2 text-left font-medium text-muted-foreground">
                      {$messages.settings.accessTokens.comment}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {#each data.currentConfig.accessTokens as token}
                    <tr class="border-b border-border last:border-b-0">
                      <td class="px-3 py-2 font-medium text-foreground">{token.name}</td>
                      <td class="px-3 py-2 text-foreground">
                        {token.enabled ? $messages.settings.gitConfig.yes : $messages.settings.gitConfig.no}
                      </td>
                      <td class="px-3 py-2 text-foreground">{token.comment || "—"}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      {/if}
    </section>
  </div>
</div>
