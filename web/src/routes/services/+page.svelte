<script lang="ts">
  import type { ActionData, PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, runtimeStatusTone } from '$lib/presenters';

  export let data: PageData;
  export let form: ActionData;

  function workspaceTone(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) {
      return 'outline';
    }
    if (runtimeStatus === 'needs_validation') {
      return 'secondary';
    }
    return runtimeStatusTone(runtimeStatus || 'unknown');
  }

  function workspaceStatus(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) {
      return 'no meta';
    }
    if (runtimeStatus === 'needs_validation') {
      return 'meta draft';
    }
    return runtimeStatus || 'unknown';
  }
</script>

<div class="page-shell">
  <Card class="border-border/70 bg-card/95">
    <CardHeader class="gap-4">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="space-y-1">
          <div class="space-y-1">
            <h1 class="page-title">Service workspace</h1>
            <p class="page-description">Declared services and uninitialized folders.</p>
          </div>
        </div>
        <Badge variant="outline">{data.services.length}</Badge>
      </div>

      {#if data.repoHead}
        <form method="POST" class="flex flex-wrap items-end gap-3 rounded-lg border border-border/70 bg-background/80 p-4">
          <input type="hidden" name="baseRevision" value={data.repoHead.headRevision} />
          <label class="flex min-w-[260px] flex-1 flex-col gap-2 text-sm">
            <span class="font-medium text-foreground">New service folder</span>
            <Input name="folder" value={form?.folder ?? ''} placeholder="my-service" />
          </label>
          <Button type="submit" formaction="?/create">Create service</Button>
        </form>
      {/if}

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>Load failed</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}

      {#if form?.error}
        <Alert variant="destructive">
          <AlertTitle>Create failed</AlertTitle>
          <AlertDescription>{form.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
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
                <TableCell class="text-muted-foreground">
                  {#if service.updatedAt}
                    {formatTimestamp(service.updatedAt)}
                  {:else if service.hasMeta}
                    Meta exists, not declared yet
                  {:else}
                    No meta file yet
                  {/if}
                </TableCell>
              </TableRow>
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">No services loaded.</div>
      {/if}
    </CardContent>
  </Card>
</div>
