<script lang="ts">
  import type { ActionData, PageData } from './$types';

  import { Plus } from 'lucide-svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import * as Popover from '$lib/components/ui/popover';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, runtimeStatusTone } from '$lib/presenters';

  interface Props {
    data: PageData;
    form: ActionData;
  }

  let { data, form }: Props = $props();

  let showDialog = $state(false);
  let newFolder = $state('');

  $effect(() => {
    newFolder = form?.folder ?? '';
  });

  function statusTone(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) return 'outline';
    if (runtimeStatus === 'needs_validation') return 'secondary';
    return runtimeStatusTone(runtimeStatus || 'unknown');
  }

  function statusText(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) return 'no meta';
    if (runtimeStatus === 'needs_validation') return 'meta draft';
    return runtimeStatus || 'unknown';
  }
</script>

<div class="page-shell">
  <Card class="border-border/70 bg-card/95">
    <CardHeader class="gap-4">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="space-y-1">
          <CardTitle class="page-title">Services</CardTitle>
        </div>
        <div class="flex items-center gap-3">
          {#if data.repoHead}
            <Popover.Root bind:open={showDialog}>
              <Popover.Trigger class="inline-flex">
                {#snippet child({ props: triggerProps })}
                  <Button type="button" {...triggerProps}>
                    <Plus class="mr-2 size-4" />
                    Create service
                  </Button>
                {/snippet}
              </Popover.Trigger>
              <Popover.Content class="w-80" align="end" sideOffset={8}>
                <form method="POST" action="?/create">
                  <div class="space-y-4">
                    <p class="text-sm font-medium">Create service</p>
                    <input type="hidden" name="baseRevision" value={data.repoHead.headRevision} />
                    <div class="grid gap-2">
                      <label for="folder" class="text-sm font-medium">Folder name</label>
                      <Input id="folder" name="folder" bind:value={newFolder} placeholder="my-service" />
                    </div>
                    {#if form?.error}
                      <Alert variant="destructive">
                        <AlertTitle>Create failed</AlertTitle>
                        <AlertDescription>{form.error}</AlertDescription>
                      </Alert>
                    {/if}
                    <div class="flex justify-end gap-2">
                      <Button type="submit" size="sm">Create</Button>
                    </div>
                  </div>
                </form>
              </Popover.Content>
            </Popover.Root>
          {/if}
          <Badge variant="outline">{data.services.length}</Badge>
        </div>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>Load failed</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
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
                  <Badge variant={statusTone(service.hasMeta, service.runtimeStatus)}>
                    {statusText(service.hasMeta, service.runtimeStatus)}
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
