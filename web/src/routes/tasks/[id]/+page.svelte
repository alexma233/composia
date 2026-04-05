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
      case 'succeeded':
        return 'border-emerald-400/30 bg-emerald-400/15 text-emerald-200';
      case 'failed':
        return 'border-rose-400/30 bg-rose-400/15 text-rose-200';
      case 'pending':
        return 'border-amber-400/30 bg-amber-400/15 text-amber-200';
      case 'running':
        return 'border-sky-400/30 bg-sky-400/15 text-sky-200';
      default:
        return 'border-slate-400/30 bg-slate-400/15 text-slate-200';
    }
  }
</script>

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
      {#if data.task}
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div class="text-sm text-slate-400">Task detail</div>
            <h1 class="mt-1 text-3xl font-semibold text-white">{data.task.type}</h1>
            <div class="mt-2 text-sm text-slate-400">
              {data.task.taskId} · {data.task.serviceName || 'system task'} · {data.task.nodeId || 'n/a'}
            </div>
          </div>
          <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(data.task.status)}`}>
            {data.task.status}
          </div>
        </div>

        <div class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Source</div>
            <div class="mt-2 text-sm text-white">{data.task.source || 'N/A'}</div>
          </div>
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Triggered by</div>
            <div class="mt-2 text-sm text-white">{data.task.triggeredBy || 'N/A'}</div>
          </div>
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Created</div>
            <div class="mt-2 text-sm text-white">{formatTimestamp(data.task.createdAt)}</div>
          </div>
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Finished</div>
            <div class="mt-2 text-sm text-white">{formatTimestamp(data.task.finishedAt)}</div>
          </div>
        </div>

        <div class="mt-6 grid gap-4 xl:grid-cols-2">
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Repo revision</div>
            <div class="mt-2 break-all text-sm text-white">{data.task.repoRevision || 'N/A'}</div>
          </div>
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Result revision</div>
            <div class="mt-2 break-all text-sm text-white">{data.task.resultRevision || 'N/A'}</div>
          </div>
        </div>

        <div class="mt-4 rounded-2xl border border-white/8 bg-slate-950/45 p-4">
          <div class="text-xs uppercase tracking-[0.2em] text-slate-500">Log path</div>
          <div class="mt-2 break-all text-sm text-white">{data.task.logPath || 'N/A'}</div>
        </div>

        {#if data.task.errorSummary}
          <div class="mt-4 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
            {data.task.errorSummary}
          </div>
        {/if}
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
      <h2 class="mb-4 text-xl font-medium text-white">Task steps</h2>
      <div class="space-y-3">
        {#each data.task?.steps ?? [] as step}
          <div class="rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="text-sm font-medium text-white">{step.stepName}</div>
              <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(step.status)}`}>{step.status}</div>
            </div>
            <div class="mt-2 text-sm text-slate-400">
              {formatTimestamp(step.startedAt)} to {formatTimestamp(step.finishedAt)}
            </div>
          </div>
        {/each}
        {#if !(data.task?.steps?.length ?? 0)}
          <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">No task steps loaded.</div>
        {/if}
      </div>
    </section>
  </div>
</div>
