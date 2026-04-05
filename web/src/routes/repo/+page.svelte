<script lang="ts">
  import type { ActionData } from './$types';
  import type { PageData } from './$types';

  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';

  export let data: PageData;
  export let form: ActionData;

  function formValue(key: 'commitMessage' | 'content') {
    if (!form || typeof form !== 'object') {
      return '';
    }
    const objectForm = form as Record<string, unknown>;
    const value = objectForm[key];
    return typeof value === 'string' ? value : '';
  }

  function parentPath(path: string) {
    if (!path) return '';
    const parts = path.split('/').filter(Boolean);
    parts.pop();
    return parts.join('/');
  }
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="grid gap-6 xl:grid-cols-[0.9fr_1.1fr]">
    <section class="rounded-lg border bg-card p-6 shadow-xs">
       <div class="mb-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-2xl font-semibold">Repo</h1>
            <p class="text-sm text-muted-foreground">Minimal Git-backed repo browser through the controller API.</p>
          </div>
          {#if data.head?.hasRemote}
            <form method="POST">
              <Button type="submit" formaction="?/sync" variant="outline">Sync repo</Button>
            </form>
          {/if}
        </div>
      </div>

      {#if data.head}
        <div class="mb-6 rounded-lg border bg-background p-4 text-sm text-muted-foreground">
          <div>Branch: <span class="text-foreground">{data.head.branch || 'HEAD'}</span></div>
          <div class="mt-1 break-all">Revision: <span class="text-foreground">{data.head.headRevision}</span></div>
          <div class="mt-1">Worktree: <span class="text-foreground">{data.head.cleanWorktree ? 'clean' : 'dirty'}</span></div>
          <div class="mt-1">Remote: <span class="text-foreground">{data.head.hasRemote ? 'configured' : 'local only'}</span></div>
          <div class="mt-1">Sync: <span class="text-foreground">{data.head.syncStatus || 'unknown'}</span></div>
          {#if data.head.lastSuccessfulPullAt}
            <div class="mt-1">Last pull: <span class="text-foreground">{data.head.lastSuccessfulPullAt}</span></div>
          {/if}
          {#if data.head.lastSyncError}
            <div class="mt-2 rounded border border-destructive/20 bg-destructive/10 px-3 py-2 text-destructive">
              {data.head.lastSyncError}
            </div>
          {/if}
        </div>
      {/if}

      {#if data.error}
        <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">{data.error}</div>
      {/if}

      <div class="mb-3 flex items-center justify-between gap-3 text-sm text-muted-foreground">
        <span>Path: {data.path || '/'}</span>
        {#if data.path}
          <a href={`/repo?path=${encodeURIComponent(parentPath(data.path))}`} class="inline-flex h-8 items-center rounded-md border bg-background px-3 text-xs transition-colors hover:bg-muted/40">Up</a>
        {/if}
      </div>

      <div class="space-y-2">
        {#each data.entries as entry}
          {#if entry.isDir}
            <a href={`/repo?path=${encodeURIComponent(entry.path)}`} class="block rounded-lg border bg-background px-4 py-3 text-sm transition-colors hover:bg-muted/40">
              {entry.name}/
            </a>
          {:else}
            <a href={`/repo?path=${encodeURIComponent(data.path)}&file=${encodeURIComponent(entry.path)}`} class="block rounded-lg border bg-background px-4 py-3 text-sm transition-colors hover:bg-muted/40">
              {entry.name}
            </a>
          {/if}
        {/each}

        {#if !data.entries.length}
          <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No entries loaded.</div>
        {/if}
      </div>
    </section>

    <section class="rounded-lg border bg-card p-6 shadow-xs">
      <div class="mb-5">
        <h2 class="text-xl font-medium">File preview</h2>
        <p class="text-sm text-muted-foreground">Selected file content from the controller repo API.</p>
      </div>

      {#if data.file}
        <div class="mb-4 text-sm text-muted-foreground">{data.file.path}</div>
        {#if form?.error}
          <div class="mb-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">{form.error}</div>
        {/if}
        <form method="POST" class="space-y-4">
          <input type="hidden" name="path" value={data.file.path} />
          <input type="hidden" name="baseRevision" value={data.head?.headRevision ?? ''} />
          <input type="hidden" name="/action" value="save" />
          <label class="block space-y-2 text-sm">
            <span>Commit message</span>
            <Input
              name="commitMessage"
              value={formValue('commitMessage')}
            />
          </label>
          <label class="block space-y-2 text-sm">
            <span>Content</span>
            <Textarea
              name="content"
              rows="24"
              value={formValue('content') || data.file.content}
              class="font-mono text-xs leading-6"
            />
          </label>
          <Button type="submit" formaction="?/save">
            Save file
          </Button>
        </form>
      {:else}
        <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-12 text-sm text-muted-foreground">
          Select a file from the left pane to preview it.
        </div>
      {/if}
    </section>
  </div>
</div>
