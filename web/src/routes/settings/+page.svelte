<script lang="ts">
  import { RefreshCw } from 'lucide-svelte';
  import { toast } from 'svelte-sonner';

  import type { PageData } from './$types';

  import ThemeControls from '$lib/components/app/theme-controls.svelte';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  export let data: PageData;

  let syncing = false;
  let syncError = '';
  let syncResult: {
    headRevision?: string;
    syncStatus?: string;
    lastSyncError?: string;
    lastSuccessfulPullAt?: string;
  } | null = null;

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
      toast.success('Repo synced successfully');
    } catch (error) {
      syncError = error instanceof Error ? error.message : 'Failed to sync repo.';
    } finally {
      syncing = false;
    }
  }

  $: displayHeadRevision = syncResult?.headRevision ?? data.repoHead?.headRevision ?? '';
  $: displaySyncStatus = syncResult?.syncStatus ?? data.repoHead?.syncStatus ?? 'unknown';
  $: displayLastSyncError = syncResult?.lastSyncError ?? data.repoHead?.lastSyncError ?? '';
  $: displayLastPull = syncResult?.lastSuccessfulPullAt ?? data.repoHead?.lastSuccessfulPullAt ?? 'Never';
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="space-y-1">
        <CardTitle class="page-title">Environment and appearance</CardTitle>
        <CardDescription class="page-description">Local theme preferences and controller metadata.</CardDescription>
      </CardHeader>
    </Card>

    {#if data.error}
      <Alert variant="destructive">
        <AlertTitle>Load failed</AlertTitle>
        <AlertDescription>{data.error}</AlertDescription>
      </Alert>
    {/if}

    <section class="grid gap-6 lg:grid-cols-2">
      <Card class="border-border/70 bg-card/95">
        <CardHeader class="space-y-1">
          <CardTitle class="section-title">Appearance</CardTitle>
          <CardDescription class="section-description">Theme and accent for this browser.</CardDescription>
        </CardHeader>
        <CardContent>
          <ThemeControls />
        </CardContent>
      </Card>

      <Card class="border-border/70 bg-card/95">
        <CardHeader class="space-y-1">
          <CardTitle class="section-title">Controller</CardTitle>
          <CardDescription class="section-description">Current controller runtime paths.</CardDescription>
        </CardHeader>
        <CardContent>
          {#if data.system}
            <dl class="kv-grid">
              <div>
                <dt>Version</dt>
                <dd>{data.system.version}</dd>
              </div>
              <div>
                <dt>Controller address</dt>
                <dd class="break-all">{data.system.controllerAddr}</dd>
              </div>
              <div>
                <dt>Repo dir</dt>
                <dd class="break-all">{data.system.repoDir}</dd>
              </div>
              <div>
                <dt>State dir</dt>
                <dd class="break-all">{data.system.stateDir}</dd>
              </div>
              <div>
                <dt>Log dir</dt>
                <dd class="break-all">{data.system.logDir}</dd>
              </div>
            </dl>
          {:else}
            <div class="empty-state">No controller data loaded.</div>
          {/if}
        </CardContent>
      </Card>

      <Card class="border-border/70 bg-card/95 lg:col-span-2">
        <CardHeader class="flex flex-row items-center justify-between gap-3">
          <div class="space-y-1">
            <CardTitle class="section-title">Repo sync</CardTitle>
            <CardDescription class="section-description">Current revision and sync health.</CardDescription>
          </div>
          <Button type="button" variant="outline" size="sm" on:click={syncRepo} disabled={syncing}>
            <RefreshCw class="mr-2 size-4" />
            {syncing ? 'Syncing...' : 'Sync Repo'}
          </Button>
        </CardHeader>
        <CardContent class="space-y-4">
          {#if syncError}
            <Alert variant="destructive">
              <AlertTitle>Sync failed</AlertTitle>
              <AlertDescription>{syncError}</AlertDescription>
            </Alert>
          {/if}

          {#if data.repoHead || syncResult}
            <dl class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Branch</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead?.branch || 'HEAD'}</dd>
              </div>
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Sync status</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{displaySyncStatus}</dd>
              </div>
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Worktree</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead?.cleanWorktree ? 'clean' : 'dirty'}</dd>
              </div>
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Last pull</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{displayLastPull}</dd>
              </div>
            </dl>

            <div class="rounded-lg border border-border/70 bg-background/80 p-4">
              <div class="metric-label">Revision</div>
              <div class="mt-2 break-all text-sm text-foreground">{displayHeadRevision}</div>
            </div>

            {#if displayLastSyncError}
              <Alert variant="destructive">
                <AlertTitle>Last sync error</AlertTitle>
                <AlertDescription>{displayLastSyncError}</AlertDescription>
              </Alert>
            {/if}
          {:else}
            <div class="empty-state">No repo state loaded.</div>
          {/if}
        </CardContent>
      </Card>
    </section>
  </div>
</div>
