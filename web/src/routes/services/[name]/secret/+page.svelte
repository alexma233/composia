<script lang="ts">
  import type { ActionData, PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';

  export let data: PageData;
  export let form: ActionData;
</script>

<div class="page-shell max-w-4xl">
  <Card class="border-border/70 bg-card/95">
    <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">{data.service?.name ?? 'Secret editor'}</CardTitle>
            <CardDescription class="page-description">Encrypted dotenv content for the service.</CardDescription>
          </div>
        {#if data.workspace}
          <a
            href={`/services/${data.workspace.folder}`}
            class="inline-flex h-9 items-center rounded-md border border-border bg-background px-4 text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
          >
            Back to service
          </a>
        {/if}
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>Load failed</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}

      {#if form?.error}
        <Alert variant="destructive">
          <AlertTitle>Save failed</AlertTitle>
          <AlertDescription>{form.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
      {#if data.secret && data.head}
        <form method="POST" class="space-y-4">
          <input type="hidden" name="baseRevision" value={data.head.headRevision} />
          <label class="block space-y-2 text-sm">
            <span class="font-medium text-foreground">Commit message</span>
            <Input name="commitMessage" value={form?.commitMessage ?? ''} />
          </label>
          <label class="block space-y-2 text-sm">
            <span class="font-medium text-foreground">Secret dotenv content</span>
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
    </CardContent>
  </Card>
</div>
