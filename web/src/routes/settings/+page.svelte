<script lang="ts">
  import type { PageData } from './$types';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-lg border bg-card p-6 shadow-xs">
      <h1 class="text-2xl font-semibold">Settings</h1>
      <p class="mt-2 text-sm text-muted-foreground">
        Controller environment and repo sync state for the current installation.
      </p>
    </section>

    {#if data.error}
      <section class="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
        {data.error}
      </section>
    {/if}

    <section class="grid gap-6 lg:grid-cols-2">
      <article class="rounded-lg border bg-card p-6 shadow-xs">
        <h2 class="text-lg font-medium">Controller</h2>
        {#if data.system}
          <dl class="mt-4 space-y-3 text-sm text-muted-foreground">
            <div>
              <dt>Version</dt>
              <dd class="text-foreground">{data.system.version}</dd>
            </div>
            <div>
              <dt>Controller address</dt>
              <dd class="break-all text-foreground">{data.system.controllerAddr}</dd>
            </div>
            <div>
              <dt>Repo dir</dt>
              <dd class="break-all text-foreground">{data.system.repoDir}</dd>
            </div>
            <div>
              <dt>State dir</dt>
              <dd class="break-all text-foreground">{data.system.stateDir}</dd>
            </div>
            <div>
              <dt>Log dir</dt>
              <dd class="break-all text-foreground">{data.system.logDir}</dd>
            </div>
          </dl>
        {:else}
          <div class="mt-4 text-sm text-muted-foreground">No controller data loaded.</div>
        {/if}
      </article>

      <article class="rounded-lg border bg-card p-6 shadow-xs">
        <h2 class="text-lg font-medium">Repo sync</h2>
        {#if data.repoHead}
          <dl class="mt-4 space-y-3 text-sm text-muted-foreground">
            <div>
              <dt>Branch</dt>
              <dd class="text-foreground">{data.repoHead.branch || 'HEAD'}</dd>
            </div>
            <div>
              <dt>Revision</dt>
              <dd class="break-all text-foreground">{data.repoHead.headRevision}</dd>
            </div>
            <div>
              <dt>Sync status</dt>
              <dd class="text-foreground">{data.repoHead.syncStatus || 'unknown'}</dd>
            </div>
            <div>
              <dt>Worktree</dt>
              <dd class="text-foreground">{data.repoHead.cleanWorktree ? 'clean' : 'dirty'}</dd>
            </div>
            {#if data.repoHead.lastSuccessfulPullAt}
              <div>
                <dt>Last successful pull</dt>
                <dd class="text-foreground">{data.repoHead.lastSuccessfulPullAt}</dd>
              </div>
            {/if}
          </dl>
          {#if data.repoHead.lastSyncError}
            <div class="mt-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
              {data.repoHead.lastSyncError}
            </div>
          {/if}
        {:else}
          <div class="mt-4 text-sm text-muted-foreground">No repo state loaded.</div>
        {/if}
      </article>
    </section>
  </div>
</div>
