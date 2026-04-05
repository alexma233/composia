<script lang="ts">
  import type { PageData } from './$types';

  export let data: PageData;

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
        <h1 class="text-2xl font-semibold text-white">Nodes</h1>
        <p class="text-sm text-slate-400">Configured nodes and live heartbeat state.</p>
      </div>
      <span class="rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs text-slate-300">
        {data.nodes.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
        {data.error}
      </div>
    {/if}

    <div class="space-y-3">
      {#each data.nodes as node}
        <a href={`/nodes/${node.nodeId}`} class="block rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4 transition hover:border-sky-400/30">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-base font-medium text-white">{node.displayName}</div>
              <div class="text-sm text-slate-400">{node.nodeId}</div>
            </div>
            <div class={`rounded-full border px-3 py-1 text-xs ${node.isOnline ? 'border-emerald-400/30 bg-emerald-400/15 text-emerald-200' : 'border-slate-400/30 bg-slate-400/15 text-slate-200'}`}>
              {node.isOnline ? 'online' : 'offline'}
            </div>
          </div>
          <div class="mt-3 text-sm text-slate-400">Last heartbeat: {formatTimestamp(node.lastHeartbeat)}</div>
        </a>
      {/each}

      {#if !data.nodes.length}
        <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">
          No nodes loaded.
        </div>
      {/if}
    </div>
  </div>
</div>
