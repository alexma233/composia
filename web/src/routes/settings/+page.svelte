<script lang="ts">
  import type { PageData } from './$types';

  import ThemeControls from '$lib/components/app/theme-controls.svelte';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  export let data: PageData;
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
        <CardHeader class="space-y-1">
          <CardTitle class="section-title">Repo sync</CardTitle>
          <CardDescription class="section-description">Current revision and sync health.</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          {#if data.repoHead}
            <dl class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Branch</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead.branch || 'HEAD'}</dd>
              </div>
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Sync status</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead.syncStatus || 'unknown'}</dd>
              </div>
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Worktree</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">{data.repoHead.cleanWorktree ? 'clean' : 'dirty'}</dd>
              </div>
              <div class="surface-subtle rounded-lg border border-border/70 p-4">
                <dt class="metric-label">Last pull</dt>
                <dd class="mt-2 text-sm font-medium text-foreground">
                  {data.repoHead.lastSuccessfulPullAt || 'Never'}
                </dd>
              </div>
            </dl>

            <div class="rounded-lg border border-border/70 bg-background/80 p-4">
              <div class="metric-label">Revision</div>
              <div class="mt-2 break-all text-sm text-foreground">{data.repoHead.headRevision}</div>
            </div>

            {#if data.repoHead.lastSyncError}
              <Alert variant="destructive">
                <AlertTitle>Last sync error</AlertTitle>
                <AlertDescription>{data.repoHead.lastSyncError}</AlertDescription>
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
