<script lang="ts">
  import type { ActionData, PageData } from './$types';

  export let data: PageData;
  export let form: ActionData;
</script>

<div class="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur">
    <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
      <div>
        <div class="text-sm text-slate-400">Service secrets</div>
        <h1 class="mt-1 text-3xl font-semibold text-white">{data.service?.name ?? 'Secret editor'}</h1>
      </div>
      {#if data.service}
        <a href={`/services/${data.service.name}`} class="rounded-full border border-white/10 bg-slate-950/45 px-4 py-2 text-sm text-slate-200 transition hover:border-sky-400/30">Back to service</a>
      {/if}
    </div>

    {#if data.error}
      <div class="mb-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">{data.error}</div>
    {/if}

    {#if form?.error}
      <div class="mb-6 rounded-2xl border border-rose-400/20 bg-rose-400/10 p-4 text-sm text-rose-100">{form.error}</div>
    {/if}

    {#if data.secret && data.head}
      <form method="POST" class="space-y-4">
        <input type="hidden" name="baseRevision" value={data.head.headRevision} />
        <label class="block space-y-2 text-sm text-slate-300">
          <span>Commit message</span>
          <input
            name="commitMessage"
            value={form?.commitMessage ?? ''}
            class="w-full rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3 text-sm text-white outline-none"
          />
        </label>
        <label class="block space-y-2 text-sm text-slate-300">
          <span>Secret dotenv content</span>
          <textarea
            name="content"
            rows="18"
            class="w-full rounded-2xl border border-white/10 bg-slate-950/65 p-4 font-mono text-xs leading-6 text-slate-200 outline-none"
          >{form?.content ?? data.secret.content}</textarea>
        </label>
        <button formaction="?/save" class="rounded-full border border-sky-400/30 bg-sky-400/15 px-4 py-2 text-sm text-sky-100 transition hover:bg-sky-400/20">
          Save secret
        </button>
      </form>
    {/if}
  </div>
</div>
