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
      {#if data.node}
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div class="text-sm text-slate-400">Node detail</div>
            <h1 class="mt-1 text-3xl font-semibold text-white">{data.node.displayName}</h1>
            <div class="mt-2 text-sm text-slate-400">
              {data.node.nodeId} · last heartbeat {formatTimestamp(data.node.lastHeartbeat)}
            </div>
          </div>
          <div class={`rounded-full border px-3 py-1 text-xs ${data.node.isOnline ? 'border-emerald-400/30 bg-emerald-400/15 text-emerald-200' : 'border-slate-400/30 bg-slate-400/15 text-slate-200'}`}>
            {data.node.isOnline ? 'online' : 'offline'}
          </div>
        </div>
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
      <h2 class="mb-4 text-xl font-medium text-white">Recent node tasks</h2>
      <div class="space-y-3">
        {#each data.tasks as task}
          <a href={`/tasks/${task.taskId}`} class="block rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4 transition hover:border-sky-400/30">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div class="text-sm font-medium text-white">{task.type}</div>
                <div class="text-xs text-slate-400">{task.serviceName || 'system task'}</div>
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
    </section>
  </div>
</div>
