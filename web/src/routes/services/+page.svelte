<script lang="ts">
  import type { PageData } from './$types';

  export let data: PageData;

  function badgeClass(status: string) {
    switch (status) {
      case 'running':
        return 'border-emerald-400/30 bg-emerald-400/15 text-emerald-200';
      case 'stopped':
        return 'border-slate-400/30 bg-slate-400/15 text-slate-200';
      case 'error':
        return 'border-rose-400/30 bg-rose-400/15 text-rose-200';
      default:
        return 'border-amber-400/30 bg-amber-400/15 text-amber-200';
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
        <h1 class="text-2xl font-semibold text-white">Services</h1>
        <p class="text-sm text-slate-400">Declared services and runtime state from the controller.</p>
      </div>
      <span class="rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs text-slate-300">
        {data.services.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">
        {data.error}
      </div>
    {/if}

    <div class="space-y-3">
      {#each data.services as service}
        <a href={`/services/${service.name}`} class="block rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4 transition hover:border-sky-400/30">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-base font-medium text-white">{service.name}</div>
              <div class="text-sm text-slate-400">Updated {formatTimestamp(service.updatedAt)}</div>
            </div>
            <div class={`rounded-full border px-3 py-1 text-xs ${badgeClass(service.runtimeStatus)}`}>
              {service.runtimeStatus}
            </div>
          </div>
        </a>
      {/each}

      {#if !data.services.length}
        <div class="rounded-2xl border border-dashed border-white/12 bg-slate-950/35 px-4 py-8 text-sm text-slate-400">
          No services loaded.
        </div>
      {/if}
    </div>
  </div>
</div>
