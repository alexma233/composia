<script lang="ts">
  import type { PageData } from './$types';

  export let data: PageData;

  function formatTimestamp(value: string) {
    if (!value) return 'N/A';
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
  }

  function badgeClass(status: string) {
    switch (status) {
      case 'running':
      case 'succeeded':
        return 'border-emerald-400/30 bg-emerald-400/15 text-emerald-200';
      case 'failed':
      case 'error':
        return 'border-rose-400/30 bg-rose-400/15 text-rose-200';
      case 'pending':
        return 'border-amber-400/30 bg-amber-400/15 text-amber-200';
      default:
        return 'border-slate-400/30 bg-slate-400/15 text-slate-200';
    }
  }
</script>

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
      {#if data.service}
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div class="text-sm text-slate-400">Service detail</div>
            <h1 class="mt-1 text-3xl font-semibold text-white">{data.service.name}</h1>
            <div class="mt-2 text-sm text-slate-400">
              Node {data.service.node} · updated {formatTimestamp(data.service.updatedAt)}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <a href={`/services/${data.service.name}/secret`} class="rounded-full border border-white/10 bg-slate-950/45 px-4 py-2 text-sm text-slate-200 transition hover:border-sky-400/30">Edit secret</a>
            <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(data.service.runtimeStatus)}`}>
              {data.service.runtimeStatus}
            </div>
          </div>
        </div>
      {:else}
        <h1 class="text-3xl font-semibold text-white">Service detail</h1>
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="grid gap-6 xl:grid-cols-2">
      <article class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
        <h2 class="mb-4 text-xl font-medium text-white">Recent tasks</h2>
        <div class="space-y-3">
          {#each data.tasks as task}
            <a href={`/tasks/${task.taskId}`} class="block rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4 transition hover:border-sky-400/30">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="text-sm font-medium text-white">{task.type}</div>
                  <div class="text-xs text-slate-400">{task.taskId}</div>
                </div>
                <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(task.status)}`}>{task.status}</div>
              </div>
              <div class="mt-2 text-sm text-slate-400">Created {formatTimestamp(task.createdAt)}</div>
            </a>
          {/each}
          {#if !data.tasks.length}
            <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">No tasks loaded.</div>
          {/if}
        </div>
      </article>

      <article class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
        <h2 class="mb-4 text-xl font-medium text-white">Recent backups</h2>
        <div class="space-y-3">
          {#each data.backups as backup}
            <div class="rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="text-sm font-medium text-white">{backup.dataName}</div>
                  <div class="text-xs text-slate-400">{backup.backupId}</div>
                </div>
                <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(backup.status)}`}>{backup.status}</div>
              </div>
              <div class="mt-2 text-sm text-slate-400">Finished {formatTimestamp(backup.finishedAt || backup.startedAt)}</div>
            </div>
          {/each}
          {#if !data.backups.length}
            <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">No backups loaded.</div>
          {/if}
        </div>
      </article>
    </section>
  </div>
</div>
