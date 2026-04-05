<script lang="ts">
  import '../app.css';

  const links = [
    { href: '/', label: 'Overview' },
    { href: '/services', label: 'Services' },
    { href: '/nodes', label: 'Nodes' },
    { href: '/tasks', label: 'Tasks' },
    { href: '/backups', label: 'Backups' },
    { href: '/repo', label: 'Repo' }
  ];
</script>

<div class="min-h-screen bg-transparent text-slate-100">
  <header class="border-b border-white/8 bg-slate-950/35 backdrop-blur">
    <div class="mx-auto flex max-w-7xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
      <a href="/" class="text-sm font-semibold uppercase tracking-[0.28em] text-sky-300">Composia</a>
      <nav class="flex flex-wrap gap-2 text-sm text-slate-300">
        {#each links as link}
          <a
            href={link.href}
            class="rounded-full border border-white/10 bg-white/5 px-3 py-1.5 transition hover:border-sky-400/30 hover:text-white"
          >
            {link.label}
          </a>
        {/each}
      </nav>
    </div>
  </header>

  <slot />
</div>
