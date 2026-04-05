<script lang="ts">
  import type { ActionData, PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, runtimeStatusTone } from '$lib/presenters';

  export let data: PageData;
  export let form: ActionData;

  function workspaceTone(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) {
      return 'outline';
    }
    return runtimeStatusTone(runtimeStatus || 'unknown');
  }

  function workspaceStatus(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) {
      return 'uninitialized';
    }
    return runtimeStatus || 'unknown';
  }
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-lg border bg-card p-6 shadow-xs">
    <div class="mb-6 flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Services</h1>
        <p class="text-sm text-muted-foreground">Top-level service folders, including folders that are not initialized yet.</p>
      </div>
      <span class="rounded-md border bg-muted/50 px-2.5 py-1 text-xs text-muted-foreground">
        {data.services.length} loaded
      </span>
    </div>

    {#if data.repoHead}
      <form method="POST" class="mb-6 flex flex-wrap items-end gap-3 rounded-lg border bg-background p-4">
        <input type="hidden" name="baseRevision" value={data.repoHead.headRevision} />
        <label class="flex min-w-[260px] flex-1 flex-col gap-2 text-sm">
          <span>New service folder</span>
          <Input name="folder" value={form?.folder ?? ''} placeholder="my-service" />
        </label>
        <Button type="submit" formaction="?/create">Create service</Button>
      </form>
    {/if}

    {#if data.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
        {data.error}
      </div>
    {/if}

    {#if form?.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
        {form.error}
      </div>
    {/if}

    {#if data.services.length}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Service</TableHead>
            <TableHead>Folder</TableHead>
            <TableHead>Status</TableHead>
            <TableHead class="w-52">Updated</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {#each data.services as service}
            <TableRow>
              <TableCell>
                <a href={`/services/${service.folder}`} class="font-medium hover:text-primary">{service.displayName}</a>
                {#if service.serviceName && service.serviceName !== service.folder}
                  <div class="text-xs text-muted-foreground">meta: {service.serviceName}</div>
                {/if}
              </TableCell>
              <TableCell class="text-muted-foreground">{service.folder}</TableCell>
              <TableCell>
                <Badge variant={workspaceTone(service.hasMeta, service.runtimeStatus)}>
                  {workspaceStatus(service.hasMeta, service.runtimeStatus)}
                </Badge>
              </TableCell>
              <TableCell class="text-muted-foreground">{service.updatedAt ? formatTimestamp(service.updatedAt) : 'Not initialized'}</TableCell>
            </TableRow>
          {/each}
        </TableBody>
      </Table>
    {:else}
      <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
        No services loaded.
      </div>
    {/if}
  </div>
</div>
