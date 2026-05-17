<script lang="ts">
  import { invalidateAll } from "$app/navigation";
  import { RefreshCw } from "lucide-svelte";
  import { onMount } from "svelte";
  import { toast } from "svelte-sonner";

  import type { PageData } from "./$types";
  import { actionErrorMessage, globalCapability } from "$lib/capabilities";
  import { messages } from "$lib/i18n";

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

  let commitPageSize = 10;
  let commits = $state(data.initialCommits?.commits ?? []);
  let commitsCursor = $state(data.initialCommits?.nextCursor ?? "");
  let loadingCommits = $state(false);
  let commitsError = $state("");
  let hasCommits = $derived(commits.length > 0 || !!commitsCursor);

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
      await invalidateAll();
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

  async function loadMoreCommits() {
    if (!commitsCursor || loadingCommits) {
      return;
    }

    loadingCommits = true;
    commitsError = "";

    try {
      const params = new URLSearchParams({
        pageSize: String(commitPageSize),
        cursor: commitsCursor,
      });
      const response = await fetch(`/settings/commits?${params}`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(
          actionErrorMessage(payload, $messages, $messages.settings.repoSync.commitFailed),
        );
      }

      commits = [...commits, ...(payload.commits ?? [])];
      commitsCursor = payload.nextCursor ?? "";
    } catch (error) {
      commitsError =
        error instanceof Error
          ? error.message
          : $messages.settings.repoSync.commitFailed;
    } finally {
      loadingCommits = false;
    }
  }

  onMount(() => startPolling(() => invalidateAll(), { intervalMs: 5000 }));
</script>

<svelte:head>
  <title>{$messages.settings.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <section class="grid gap-6 lg:grid-cols-2">
      <Card>
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
        <CardContent>
          <CardTitle class="section-title" level="2">{$messages.settings.appearance.title}</CardTitle>
          <ThemeControls />
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="section-header">
          <div class="section-heading">
            <CardTitle class="section-title" level="2"
              >{$messages.settings.controller.title}</CardTitle
            >
          </div>
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
        </CardHeader>
        <CardContent class="space-y-4">
          {#if reloadControllerError}
            <Alert variant="destructive">
              <AlertTitle>{$messages.settings.controller.reloadFailed}</AlertTitle>
              <AlertDescription>{reloadControllerError}</AlertDescription>
            </Alert>
          {/if}

          {#if reloadAccepted === true}
            <Alert>
              <AlertTitle>{$messages.common.success}</AlertTitle>
              <AlertDescription
                >{$messages.settings.controller.reloadAccepted}</AlertDescription
              >
            </Alert>
          {/if}

          {#if data.system}
            <dl class="grid gap-4 sm:grid-cols-2">
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
            <div class="empty-state">
              {$messages.settings.controller.noData}
            </div>
          {/if}
        </CardContent>
      </Card>

      <Card class="lg:col-span-2">
        <CardHeader class="section-header">
          <div class="section-heading">
            <CardTitle class="section-title" level="2"
              >{$messages.settings.repoSync.title}</CardTitle
            >
          </div>
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
        </CardHeader>
        <CardContent class="space-y-4">
          {#if syncError}
            <Alert variant="destructive">
              <AlertTitle>{$messages.error.syncFailed}</AlertTitle>
              <AlertDescription>{syncError}</AlertDescription>
            </Alert>
          {/if}

          {#if data.repoHead || syncResult}
            <dl class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
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

            <div class="inset-card">
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
            <div class="empty-state">
              {$messages.settings.repoSync.noRepoState}
            </div>
          {/if}
        </CardContent>
      </Card>

      {#if hasCommits}
        <Card class="lg:col-span-2">
          <CardHeader class="section-header">
            <div class="section-heading">
              <CardTitle class="section-title" level="2"
                >{$messages.settings.repoSync.commitHistory}</CardTitle
              >
            </div>
          </CardHeader>
          <CardContent class="space-y-4">
            {#if commitsError}
              <Alert variant="destructive">
                <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
                <AlertDescription>{commitsError}</AlertDescription>
              </Alert>
            {/if}

            {#if commits.length > 0}
              <div class="space-y-3">
                {#each commits as commit}
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

            {#if commitsCursor}
              <Button
                type="button"
                variant="outline"
                size="sm"
                onclick={loadMoreCommits}
                disabled={loadingCommits}
                class="w-full"
              >
                {loadingCommits
                  ? $messages.settings.repoSync.loadingCommits
                  : $messages.settings.repoSync.loadMore}
              </Button>
            {/if}
          </CardContent>
        </Card>
      {/if}

      {#if rusticMaintenanceCapability.enabled}
        <Card class="lg:col-span-2">
          <CardHeader class="section-header">
            <div class="section-heading">
              <CardTitle class="section-title" level="2"
                >{$messages.settings.rustic.title}</CardTitle
              >
            </div>
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
          </CardHeader>
          <CardContent class="space-y-4">
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
      {/if}
    </section>
  </div>
</div>
