<script lang="ts">
  import type { PageData } from './$types';

  export let data: PageData;

  const statusTone = {
    running: 'bg-emerald-400/15 text-emerald-200 border-emerald-400/30',
    stopped: 'bg-slate-400/15 text-slate-200 border-slate-400/30',
    error: 'bg-rose-400/15 text-rose-200 border-rose-400/30',
    unknown: 'bg-amber-400/15 text-amber-200 border-amber-400/30',
    succeeded: 'bg-emerald-400/15 text-emerald-200 border-emerald-400/30',
    failed: 'bg-rose-400/15 text-rose-200 border-rose-400/30',
    pending: 'bg-amber-400/15 text-amber-200 border-amber-400/30',
    running_task: 'bg-sky-400/15 text-sky-200 border-sky-400/30',
    cancelled: 'bg-slate-400/15 text-slate-200 border-slate-400/30'
  } as const;

  function badgeClass(status: string) {
    if (status === 'running') {
      return statusTone.running;
    }
    return statusTone[status as keyof typeof statusTone] ?? statusTone.unknown;
  }

  function formatTimestamp(value: string) {
    if (!value) return 'N/A';
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    return parsed.toLocaleString();
  }

  function onlineSummary() {
    if (!data.dashboard) return 'No runtime data';
    return `${data.dashboard.system.onlineNodeCount}/${data.dashboard.system.configuredNodeCount} nodes online`;
  }
</script>

<svelte:head>
  <title>Composia Control Plane</title>
  <meta
    name="description"
    content="Composia controller overview with live services, nodes, and task history."
  />
</svelte:head>

<div class="mx-auto min-h-screen max-w-7xl px-4 py-8 text-slate-100 sm:px-6 lg:px-8">
  <div class="space-y-8">
    <section class="grid gap-6 lg:grid-cols-[1.3fr_0.7fr]">
      <div class="rounded-3xl border border-white/10 bg-white/5 p-6 shadow-2xl shadow-black/30 backdrop-blur">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-3">
            <p class="text-sm uppercase tracking-[0.28em] text-sky-300">Composia</p>
            <div class="space-y-2">
              <h1 class="text-3xl font-semibold text-white md:text-5xl">
                Service-first control plane overview
              </h1>
              <p class="max-w-3xl text-sm leading-7 text-slate-300 md:text-base">
                The homepage is now wired to the live controller APIs instead of the scaffold landing
                page. It surfaces system status, declared services, configured nodes, and recent
                tasks from the same runtime state used by the backend.
              </p>
            </div>
          </div>

          <div class="rounded-2xl border border-sky-400/20 bg-sky-400/10 px-4 py-3 text-right text-sm text-sky-100">
            <div class="font-medium">{data.dashboard?.system.version ?? 'Controller unavailable'}</div>
            <div class="text-sky-200/80">{onlineSummary()}</div>
          </div>
        </div>

        {#if data.error}
          <div class="mt-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
            {data.error}
          </div>
        {/if}
      </div>

      <div class="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
        <article class="rounded-3xl border border-white/10 bg-slate-950/45 p-5">
          <div class="text-sm text-slate-400">Configured nodes</div>
          <div class="mt-2 text-3xl font-semibold text-white">
            {data.dashboard?.system.configuredNodeCount ?? '-'}
          </div>
        </article>
        <article class="rounded-3xl border border-white/10 bg-slate-950/45 p-5">
          <div class="text-sm text-slate-400">Online nodes</div>
          <div class="mt-2 text-3xl font-semibold text-white">
            {data.dashboard?.system.onlineNodeCount ?? '-'}
          </div>
        </article>
        <article class="rounded-3xl border border-white/10 bg-slate-950/45 p-5">
          <div class="text-sm text-slate-400">Recent tasks shown</div>
          <div class="mt-2 text-3xl font-semibold text-white">
            {data.dashboard?.tasks.length ?? 0}
          </div>
        </article>
      </div>
    </section>

    <section class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
      <article class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
        <div class="mb-5 flex items-center justify-between gap-4">
          <div>
            <h2 class="text-xl font-medium text-white">Services</h2>
            <p class="text-sm text-slate-400">Current declared services and runtime state</p>
          </div>
          <span class="rounded-full border border-white/10 bg-slate-950/50 px-3 py-1 text-xs text-slate-300">
            {data.dashboard?.services.length ?? 0} loaded
          </span>
        </div>

        <div class="space-y-3">
          {#if data.dashboard?.services.length}
            {#each data.dashboard.services as service}
              <div class="rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div class="text-base font-medium text-white">{service.name}</div>
                    <div class="text-sm text-slate-400">
                      Updated {formatTimestamp(service.updatedAt)}
                    </div>
                  </div>
                  <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(service.runtimeStatus)}`}>
                    {service.runtimeStatus}
                  </div>
                </div>
              </div>
            {/each}
          {:else}
            <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">
              No service data loaded.
            </div>
          {/if}
        </div>
      </article>

      <div class="grid gap-6">
        <article class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
          <div class="mb-5">
            <h2 class="text-xl font-medium text-white">Nodes</h2>
            <p class="text-sm text-slate-400">Configured nodes and heartbeat state</p>
          </div>

          <div class="space-y-3">
            {#if data.dashboard?.nodes.length}
              {#each data.dashboard.nodes as node}
                <div class="rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <div class="text-base font-medium text-white">{node.displayName}</div>
                      <div class="text-sm text-slate-400">{node.nodeId}</div>
                    </div>
                    <div class={`rounded-full border px-3 py-1 text-xs ${node.isOnline ? badgeClass('running') : badgeClass('stopped')}`}>
                      {node.isOnline ? 'online' : 'offline'}
                    </div>
                  </div>
                  <div class="mt-3 text-sm text-slate-400">
                    Last heartbeat: {formatTimestamp(node.lastHeartbeat)}
                  </div>
                </div>
              {/each}
            {:else}
              <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">
                No node data loaded.
              </div>
            {/if}
          </div>
        </article>

        <article class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
          <div class="mb-5">
            <h2 class="text-xl font-medium text-white">Recent tasks</h2>
            <p class="text-sm text-slate-400">Latest task activity from the durable queue</p>
          </div>

          <div class="space-y-3">
            {#if data.dashboard?.tasks.length}
              {#each data.dashboard.tasks as task}
                <div class="rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="min-w-0">
                      <div class="truncate text-sm font-medium text-white">
                        {task.type} {task.serviceName ? `for ${task.serviceName}` : ''}
                      </div>
                      <div class="text-xs text-slate-400">
                        {task.taskId} on {task.nodeId || 'n/a'}
                      </div>
                    </div>
                    <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(task.status)}`}>
                      {task.status}
                    </div>
                  </div>
                  <div class="mt-3 text-sm text-slate-400">
                    Created {formatTimestamp(task.createdAt)}
                  </div>
                </div>
              {/each}
            {:else}
              <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">
                No task data loaded.
              </div>
            {/if}
          </div>
        </article>
      </div>
    </section>
  </div>
</div>
