<script lang="ts">
  import { RefreshCw } from 'lucide-svelte';
  import { toast } from 'svelte-sonner';

  import type { PageData } from './$types';
  import { messages } from '$lib/i18n';

  import ThemeControls from '$lib/components/app/theme-controls.svelte';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let syncing = $state(false);
  let syncError = $state('');
  let rusticBusy = $state<'forget' | 'prune' | ''>('');
  let rusticError = $state('');
  let rusticTaskId = $state('');
  let syncResult = $state<{
    headRevision?: string;
    syncStatus?: string;
    lastSyncError?: string;
    lastSuccessfulPullAt?: string;
  } | null>(null);

  async function syncRepo() {
    syncing = true;
    syncError = '';
    syncResult = null;

    try {
      const response = await fetch('/settings/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });

      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? 'Failed to sync repo.');
      }

      syncResult = {
        headRevision: payload.headRevision,
        syncStatus: payload.syncStatus,
        lastSyncError: payload.lastSyncError,
        lastSuccessfulPullAt: payload.lastSuccessfulPullAt,
      };
      toast.success($messages.settings.repoSync.syncedSuccessfully);
    } catch (error) {
      syncError = error instanceof Error ? error.message : 'Failed to sync repo.';
    } finally {
      syncing = false;
    }
  }

  async function runRusticAction(action: 'forget' | 'prune') {
    rusticBusy = action;
    rusticError = '';
    rusticTaskId = '';

    try {
      const response = await fetch(`/settings/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });

      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? `Failed to start rustic ${action}.`);
      }

      rusticTaskId = payload.taskId ?? '';
      toast.success(`Rustic ${action} started`);
    } catch (error) {
      rusticError = error instanceof Error ? error.message : `Failed to start rustic ${action}.`;
    } finally {
      rusticBusy = '';
    }
  }

  let displayHeadRevision = $derived(syncResult?.headRevision ?? data.repoHead?.headRevision ?? '');
  let displaySyncStatus = $derived(syncResult?.syncStatus ?? data.repoHead?.syncStatus ?? $messages.common.unknown);
  let displayLastSyncError = $derived(syncResult?.lastSyncError ?? data.repoHead?.lastSyncError ?? '');
  let displayLastPull = $derived(syncResult?.lastSuccessfulPullAt ?? data.repoHead?.lastSuccessfulPullAt ?? $messages.common.never);
</script>

<div class="page-shell">
  <div class="page-stack">
		<Card>
      <CardHeader class="space-y-2">
        <CardTitle class="page-title">{$messages.settings.title}</CardTitle>
        <CardDescription class="page-description">
          {$messages.settings.description}
        </CardDescription>
      </CardHeader>
    </Card>

    {#if data.error}
      <Alert variant="destructive">
        <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
        <AlertDescription>{data.error}</AlertDescription>
      </Alert>
    {/if}

    <section class="grid gap-6 lg:grid-cols-2">
			<Card>
        <CardHeader class="space-y-2">
          <CardTitle class="section-title">{$messages.settings.appearance.title}</CardTitle>
          <CardDescription class="section-description">
            {$messages.settings.appearance.description}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ThemeControls />
        </CardContent>
      </Card>

			<Card>
        <CardHeader class="space-y-2">
          <CardTitle class="section-title">{$messages.settings.controller.title}</CardTitle>
          <CardDescription class="section-description">
            {$messages.settings.controller.description}
          </CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          {#if data.system}
            <dl class="grid gap-4 sm:grid-cols-2">
              <div class="metric-card">
                <dt class="metric-label">{$messages.settings.controller.version}</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.system.version}</dd>
              </div>
              <div class="metric-card sm:col-span-2">
                <dt class="metric-label">{$messages.settings.controller.controllerAddress}</dt>
                <dd class="mt-2 break-all text-sm font-medium text-foreground">{data.system.controllerAddr}</dd>
              </div>
              <div class="inset-card">
                <dt class="metric-label">{$messages.settings.controller.repoDir}</dt>
                <dd class="mt-2 break-all text-sm text-foreground">{data.system.repoDir}</dd>
              </div>
              <div class="inset-card">
                <dt class="metric-label">{$messages.settings.controller.stateDir}</dt>
                <dd class="mt-2 break-all text-sm text-foreground">{data.system.stateDir}</dd>
              </div>
              <div class="inset-card sm:col-span-2">
                <dt class="metric-label">{$messages.settings.controller.logDir}</dt>
                <dd class="mt-2 break-all text-sm text-foreground">{data.system.logDir}</dd>
              </div>
            </dl>
          {:else}
            <div class="empty-state">{$messages.settings.controller.noData}</div>
          {/if}
        </CardContent>
      </Card>

		<Card class="lg:col-span-2">
        <CardHeader class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-2">
            <CardTitle class="section-title">{$messages.settings.repoSync.title}</CardTitle>
            <CardDescription class="section-description">
              {$messages.settings.repoSync.description}
            </CardDescription>
          </div>
          <Button type="button" variant="outline" size="sm" onclick={syncRepo} disabled={syncing}>
            <RefreshCw class="mr-2 size-4" />
            {syncing ? $messages.settings.repoSync.syncing : $messages.settings.repoSync.syncRepo}
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
                <dt class="metric-label">{$messages.settings.repoSync.branch}</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead?.branch || $messages.settings.repoSync.head}</dd>
              </div>
              <div class="metric-card">
                <dt class="metric-label">{$messages.settings.repoSync.syncStatus}</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{displaySyncStatus}</dd>
              </div>
              <div class="metric-card">
                <dt class="metric-label">{$messages.settings.repoSync.worktree}</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead?.cleanWorktree ? $messages.status.clean : $messages.status.dirty}</dd>
              </div>
              <div class="metric-card">
                <dt class="metric-label">{$messages.settings.repoSync.lastPull}</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{displayLastPull}</dd>
              </div>
            </dl>

            <div class="inset-card">
              <div class="metric-label">{$messages.settings.repoSync.revision}</div>
              <div class="mt-2 break-all text-sm text-foreground">{displayHeadRevision}</div>
            </div>

            {#if displayLastSyncError}
              <Alert variant="destructive">
                <AlertTitle>{$messages.error.lastSyncError}</AlertTitle>
                <AlertDescription>{displayLastSyncError}</AlertDescription>
              </Alert>
            {/if}
          {:else}
            <div class="empty-state">{$messages.settings.repoSync.noRepoState}</div>
          {/if}
        </CardContent>
      </Card>

		<Card class="lg:col-span-2">
        <CardHeader class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-2">
            <CardTitle class="section-title">{$messages.settings.rustic.title}</CardTitle>
            <CardDescription class="section-description">
              {$messages.settings.rustic.summary}
            </CardDescription>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={() => runRusticAction('forget')}
              disabled={rusticBusy !== ''}
            >
              {rusticBusy === 'forget' ? $messages.settings.rustic.starting : $messages.settings.rustic.forget}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={() => runRusticAction('prune')}
              disabled={rusticBusy !== ''}
            >
              {rusticBusy === 'prune' ? $messages.settings.rustic.starting : $messages.settings.rustic.prune}
            </Button>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="text-sm text-muted-foreground">
            {$messages.settings.rustic.description}
          </div>

          {#if rusticError}
            <Alert variant="destructive">
              <AlertTitle>{$messages.error.taskError}</AlertTitle>
              <AlertDescription>{rusticError}</AlertDescription>
            </Alert>
          {/if}

          {#if rusticTaskId}
            <div class="inset-card">
              <div class="metric-label">{$messages.settings.rustic.lastTask}</div>
              <div class="mt-2 break-all text-sm text-foreground">{rusticTaskId}</div>
            </div>
          {/if}
        </CardContent>
      </Card>
    </section>
  </div>
</div>
