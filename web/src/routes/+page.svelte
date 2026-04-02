<script lang="ts">
  const highlights = [
    'Service-first deployment model',
    'Single active main with multiple agents',
    'SQLite runtime state with Git-backed declarations',
    'Docker Compose execution and Caddy integration'
  ];
</script>

<svelte:head>
  <title>Composia</title>
  <meta
    name="description"
    content="Composia development scaffold for the main control plane, agent runtime, and web UI."
  />
</svelte:head>

<div class="mx-auto flex min-h-screen max-w-6xl items-center px-6 py-16 text-slate-100">
  <div class="grid gap-10 lg:grid-cols-[1.3fr_0.9fr] lg:items-center">
    <section class="space-y-6">
      <p class="text-sm uppercase tracking-[0.3em] text-sky-300">Composia</p>
      <div class="space-y-4">
        <h1 class="max-w-3xl text-4xl font-semibold leading-tight text-white md:text-6xl">
          Self-hosted service control plane with a clean Go and Bun starting point.
        </h1>
        <p class="max-w-2xl text-base leading-7 text-slate-300 md:text-lg">
          This scaffold sets up the backend entrypoint, development configs, and a SvelteKit
          frontend shell so the next step can focus on real main-agent behavior instead of project
          plumbing.
        </p>
      </div>

      <div class="flex flex-wrap gap-3 text-sm text-slate-200">
        <span class="rounded-full border border-sky-400/40 bg-sky-400/10 px-3 py-1">Go 1.25</span>
        <span class="rounded-full border border-cyan-400/40 bg-cyan-400/10 px-3 py-1">Bun</span>
        <span class="rounded-full border border-violet-400/40 bg-violet-400/10 px-3 py-1">
          SvelteKit
        </span>
        <span class="rounded-full border border-emerald-400/40 bg-emerald-400/10 px-3 py-1">
          Docker Compose
        </span>
      </div>
    </section>

    <section class="rounded-3xl border border-white/10 bg-white/5 p-6 shadow-2xl shadow-black/30 backdrop-blur">
      <div class="mb-6 flex items-center justify-between">
        <h2 class="text-lg font-medium text-white">Current focus</h2>
        <span class="rounded-full bg-emerald-400/15 px-3 py-1 text-xs text-emerald-200">
          Scaffold ready
        </span>
      </div>

      <ul class="space-y-3">
        {#each highlights as item}
          <li class="rounded-2xl border border-white/8 bg-slate-950/40 px-4 py-3 text-sm text-slate-200">
            {item}
          </li>
        {/each}
      </ul>

      <div class="mt-6 rounded-2xl border border-sky-400/20 bg-sky-400/8 p-4 text-sm text-sky-100">
        Next: add config loading, protobuf generation, and the first main-agent heartbeat.
      </div>
    </section>
  </div>
</div>
