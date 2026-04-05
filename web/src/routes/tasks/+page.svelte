<script lang="ts">
  import type { PageData } from './$types';

  export let data: PageData;

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

  function formatTimestamp(value: string) {
    if (!value) return 'N/A';
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
  }
</script>

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
    <div class="mb-6 flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold text-white">Tasks</h1>
        <p class="text-sm text-slate-400">Recent task history from the durable queue.</p>
      </div>
      <span class="rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs text-slate-300">
        {data.tasks.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
        {data.error}
      </div>
    {/if}

    <div class="space-y-3">
      {#each data.tasks as task}
        <div class="rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-base font-medium text-white">{task.type}</div>
              <div class="text-sm text-slate-400">
                {task.serviceName || 'system task'} on {task.nodeId || 'n/a'}
              </div>
            </div>
            <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(task.status)}`}>
              {task.status}
            </div>
          </div>
          <div class="mt-3 text-sm text-slate-400">
            {task.taskId} · created {formatTimestamp(task.createdAt)}
          </div>
        </div>
      {/each}

      {#if !data.tasks.length}
        <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">
          No tasks loaded.
        </div>
      {/if}
    </div>
  </div>
</div>
