<script lang="ts">
  import { invalidate } from '$app/navigation';
  import { onMount } from 'svelte';
  import type { ActionData, PageData } from './$types';

  import { Plus } from '@lucide/svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import { startPolling } from '$lib/refresh';
  import * as Popover from '$lib/components/ui/popover';
  import { Table, TableBody, TableCaption, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { getMessages } from '$lib/i18n';

  const messages = getMessages();
  import ServiceRow from '$lib/components/app/service-row.svelte';
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

  onMount(() => startPolling(() => invalidate('app:services'), { intervalMs: 5000 }));
</script>

<svelte:head>
  <title>{$messages.services.title} - {$messages.app.name}</title>
  <meta
    name="description"
    content={$messages.services.pageDescription}
  />
</svelte:head>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title" level="1">{$messages.services.title}</CardTitle>
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
                      <Input id="folder" name="folder" bind:value={newFolder} placeholder={$messages.services.files.newServiceFolderPlaceholder} />
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
          <TableCaption class="sr-only">{$messages.services.tableCaption}</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>{$messages.services.service}</TableHead>
              <TableHead>{$messages.services.folder}</TableHead>
              <TableHead>{$messages.nodes.node}</TableHead>
              <TableHead>{$messages.common.status}</TableHead>
              <TableHead class="w-52">{$messages.common.updated}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {#each data.services as service}
              <ServiceRow {service} />
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">{$messages.services.noServices}</div>
      {/if}
    </CardContent>
  </Card>
</div>
