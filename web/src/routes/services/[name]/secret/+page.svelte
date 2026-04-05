<script lang="ts">
  import type { ActionData, PageData } from './$types';

  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';

  export let data: PageData;
  export let form: ActionData;
</script>

<div class="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-lg border bg-card p-6 shadow-xs">
    <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
      <div>
        <div class="text-sm text-muted-foreground">Service secrets</div>
        <h1 class="mt-1 text-3xl font-semibold tracking-tight">{data.service?.name ?? 'Secret editor'}</h1>
      </div>
      {#if data.workspace}
        <a href={`/services/${data.workspace.folder}`} class="inline-flex h-9 items-center rounded-md border bg-background px-4 text-sm transition-colors hover:bg-muted/40">Back to service</a>
      {/if}
    </div>

    {#if data.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">{data.error}</div>
    {/if}

    {#if form?.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">{form.error}</div>
    {/if}

    {#if data.secret && data.head}
      <form method="POST" class="space-y-4">
        <input type="hidden" name="baseRevision" value={data.head.headRevision} />
        <label class="block space-y-2 text-sm">
          <span>Commit message</span>
          <Input
            name="commitMessage"
            value={form?.commitMessage ?? ''}
          />
        </label>
        <label class="block space-y-2 text-sm">
          <span>Secret dotenv content</span>
          <Textarea
            name="content"
            rows="18"
            value={form?.content ?? data.secret.content}
            class="font-mono text-xs leading-6"
          />
        </label>
        <Button type="submit" formaction="?/save">
          Save secret
        </Button>
      </form>
    {/if}
  </div>
</div>
