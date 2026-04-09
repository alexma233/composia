<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { onMount } from 'svelte';
  import type { ActionData, PageData } from './$types';

  import { Plus } from 'lucide-svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import { startPolling } from '$lib/refresh';
  import * as Popover from '$lib/components/ui/popover';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, runtimeStatusTone } from '$lib/presenters';
  import { messages } from '$lib/i18n';

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
    if (!hasMeta) return $messages.services.noMeta;
    if (runtimeStatus === 'needs_validation') return $messages.services.metaDraft;
    return runtimeStatus || $messages.common.unknown;
  }

  onMount(() => startPolling(() => invalidateAll(), { intervalMs: 5000 }));
</script>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title">{$messages.services.title}</CardTitle>
        </div>
        <div class="flex items-center gap-3">
          {#if data.repoHead}
            <Popover.Root bind:open={showDialog}>
              <Popover.Trigger class="inline-flex">
                {#snippet child({ props: triggerProps })}
                  <Button type="button" {...triggerProps}>
                    <Plus class="mr-2 size-4" />
                    {$messages.services.createService}
                  </Button>
                {/snippet}
              </Popover.Trigger>
              <Popover.Content class="w-80" align="end" sideOffset={8}>
                <form method="POST" action="?/create">
                  <div class="space-y-4">
                    <p class="text-sm font-medium">{$messages.services.createService}</p>
                    <input type="hidden" name="baseRevision" value={data.repoHead.headRevision} />
                    <div class="grid gap-2">
                      <label for="folder" class="text-sm font-medium">{$messages.services.folderName}</label>
                      <Input id="folder" name="folder" bind:value={newFolder} placeholder="my-service" />
                    </div>
                    {#if form?.error}
                      <Alert variant="destructive">
                        <AlertTitle>{$messages.error.createFailed}</AlertTitle>
                        <AlertDescription>{form.error}</AlertDescription>
                      </Alert>
                    {/if}
                    <div class="flex justify-end gap-2">
                      <Button type="submit" size="sm">{$messages.common.create}</Button>
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
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
      {#if data.services.length}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{$messages.services.service}</TableHead>
              <TableHead>{$messages.services.folder}</TableHead>
              <TableHead>{$messages.common.status}</TableHead>
              <TableHead class="w-52">{$messages.common.updated}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {#each data.services as service}
              <TableRow>
                <TableCell>
                  <a href={`/services/${service.folder}`} class="font-medium hover:text-primary">{service.displayName}</a>
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
                    {$messages.services.metaExistsNotDeclared}
                  {:else}
                    {$messages.services.noMetaFile}
                  {/if}
                </TableCell>
              </TableRow>
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">{$messages.common.noData}</div>
      {/if}
    </CardContent>
  </Card>
</div>
