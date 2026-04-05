<script lang="ts">
  import type { ActionData } from './$types';
  import type { PageData } from './$types';

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

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="grid gap-6 xl:grid-cols-[0.9fr_1.1fr]">
    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
      <div class="mb-5">
        <h1 class="text-2xl font-semibold text-white">Repo</h1>
        <p class="text-sm text-slate-400">Minimal Git-backed repo browser through the controller API.</p>
      </div>

      {#if data.head}
        <div class="mb-6 rounded-2xl border border-white/8 bg-slate-950/45 p-4 text-sm text-slate-300">
          <div>Branch: <span class="text-white">{data.head.branch || 'HEAD'}</span></div>
          <div class="mt-1 break-all">Revision: <span class="text-white">{data.head.headRevision}</span></div>
          <div class="mt-1">Worktree: <span class="text-white">{data.head.cleanWorktree ? 'clean' : 'dirty'}</span></div>
        </div>
      {/if}

      {#if data.error}
        <div class="mb-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">{data.error}</div>
      {/if}

      <div class="mb-3 flex items-center justify-between gap-3 text-sm text-slate-400">
        <span>Path: {data.path || '/'}</span>
        {#if data.path}
          <a href={`/repo?path=${encodeURIComponent(parentPath(data.path))}`} class="rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs text-slate-200">Up</a>
        {/if}
      </div>

      <div class="space-y-2">
        {#each data.entries as entry}
          {#if entry.isDir}
            <a href={`/repo?path=${encodeURIComponent(entry.path)}`} class="block rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-3 text-sm text-slate-100 transition hover:border-sky-400/30">
              {entry.name}/
            </a>
          {:else}
            <a href={`/repo?path=${encodeURIComponent(data.path)}&file=${encodeURIComponent(entry.path)}`} class="block rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-3 text-sm text-slate-100 transition hover:border-sky-400/30">
              {entry.name}
            </a>
          {/if}
        {/each}

        {#if !data.entries.length}
          <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">No entries loaded.</div>
        {/if}
      </div>
    </section>

    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
      <div class="mb-5">
        <h2 class="text-xl font-medium text-white">File preview</h2>
        <p class="text-sm text-slate-400">Selected file content from the controller repo API.</p>
      </div>

      {#if data.file}
        <div class="mb-4 text-sm text-slate-400">{data.file.path}</div>
        {#if form?.error}
          <div class="mb-4 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">{form.error}</div>
        {/if}
        <form method="POST" class="space-y-4">
          <input type="hidden" name="path" value={data.file.path} />
          <input type="hidden" name="baseRevision" value={data.head?.headRevision ?? ''} />
          <input type="hidden" name="/action" value="save" />
          <label class="block space-y-2 text-sm text-slate-300">
            <span>Commit message</span>
            <input
              name="commitMessage"
              value={formValue('commitMessage')}
              class="w-full rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3 text-sm text-white outline-none"
            />
          </label>
          <label class="block space-y-2 text-sm text-slate-300">
            <span>Content</span>
            <textarea
              name="content"
              rows="24"
              class="w-full rounded-2xl border border-white/10 bg-slate-950/65 p-4 font-mono text-xs leading-6 text-slate-200 outline-none"
            >{formValue('content') || data.file.content}</textarea>
          </label>
          <button formaction="?/save" class="rounded-full border border-sky-400/30 bg-sky-400/15 px-4 py-2 text-sm text-sky-100 transition hover:bg-sky-400/20">
            Save file
          </button>
        </form>
      {:else}
        <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-12 text-sm text-slate-400">
          Select a file from the left pane to preview it.
        </div>
      {/if}
    </section>
  </div>
</div>
