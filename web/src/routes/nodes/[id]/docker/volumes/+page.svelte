<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogOverlay,
    DialogTitle,
  } from '$lib/components/ui/dialog';
  import { formatBytes, formatDockerTimestamp } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
  import { Alert, AlertDescription } from '$lib/components/ui/alert';
  import { messages } from '$lib/i18n';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  type DockerVolumeSummary = {
    name: string;
    driver: string;
    mountpoint: string;
    scope: string;
    created: string;
    labels: Record<string, string>;
    sizeBytes: number;
    containersCount: number;
    inUse: boolean;
  };

  let searchQuery = $state('');
  let sortField = $state<'name' | 'driver' | 'created'>('name');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let volumes = $state<DockerVolumeSummary[]>([]);
  let removeBusyId = $state('');
  let removeDialogOpen = $state(false);
  let removeTarget = $state<DockerVolumeSummary | null>(null);

  $effect(() => {
    loading = !data.ready;
    loadError = data.error ?? null;
    volumes = data.volumes || [];
  });

  async function refreshVolumes() {
    if (!data.ready) {
      loading = false;
      return;
    }

    loading = true;
    loadError = null;

    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/volumes`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error || $messages.docker.volumes.failedToLoad);
      }
      volumes = payload.volumes ?? [];
    } catch (error) {
      loadError = error instanceof Error ? error.message : $messages.docker.volumes.failedToLoad;
      volumes = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void refreshVolumes();
  });

  function openRemoveDialog(volume: DockerVolumeSummary) {
    removeTarget = volume;
    removeDialogOpen = true;
  }

  async function queueVolumeRemove() {
    if (!removeTarget) {
      return;
    }

    const volume = removeTarget;
    removeBusyId = volume.name;
    removeDialogOpen = false;
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/volumes/${encodeURIComponent(volume.name)}/remove`, {
        method: 'POST'
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? $messages.docker.volumes.removeFailed);
      }
      toast.success($messages.docker.volumes.removeQueued.replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'));
      await refreshVolumes();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.volumes.removeFailed);
    } finally {
      removeBusyId = '';
      removeTarget = null;
    }
  }

  let removeDescription = $derived(
    removeTarget ? $messages.docker.volumes.removeConfirm.replace('{name}', removeTarget.name) : '',
  );

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as typeof sortField;
      sortDirection = 'asc';
    }
  }

  let filteredVolumes = $derived(volumes.filter((v) => {
    const query = searchQuery.toLowerCase();
    return (
      v.name.toLowerCase().includes(query) ||
      v.driver.toLowerCase().includes(query) ||
      v.mountpoint.toLowerCase().includes(query)
    );
  }));

  let sortedVolumes = $derived([...filteredVolumes].sort((a, b) => {
    let comparison = 0;
    switch (sortField) {
      case 'name':
        comparison = a.name.localeCompare(b.name);
        break;
      case 'driver':
        comparison = a.driver.localeCompare(b.driver);
        break;
      case 'created':
        comparison = new Date(a.created || 0).getTime() - new Date(b.created || 0).getTime();
        break;
    }
    return sortDirection === 'asc' ? comparison : -comparison;
  }));
</script>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">{$messages.docker.volumes.title}</CardTitle>
            <CardDescription class="page-description">
              {$messages.docker.volumes.title} on {data.nodeId}
              {#if !loading}
                <Badge variant="secondary" class="ml-2">{volumes.length}</Badge>
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            {$messages.common.back}
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <SearchIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder={$messages.docker.volumes.searchPlaceholder}
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
              {$messages.common.cancel}
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void refreshVolumes()} disabled={loading || !data.ready}>
            {#if loading}{$messages.common.loading}...{:else}{$messages.common.refresh}{/if}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {#if loadError}
          <Alert variant="destructive">
            <AlertDescription>{loadError}</AlertDescription>
          </Alert>
        {:else if loading}
          <div class="flex min-h-[320px] items-center justify-center">
            <div class="flex items-center gap-3 text-sm text-muted-foreground">
              <Spinner />
              <span>{$messages.docker.volumes.loadingWithUsage}</span>
            </div>
          </div>
        {:else if sortedVolumes.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.common.name} {sortField} {sortDirection} onSort={handleSort} class="w-[25%]" />
                <TableHead class="w-[10%]">{$messages.docker.volumes.driver}</TableHead>
                <TableHead class="w-[10%]">{$messages.docker.volumes.size}</TableHead>
                <TableHead class="w-[10%]">{$messages.docker.volumes.usage}</TableHead>
                <TableHead class="w-[25%]">{$messages.docker.volumes.mountpoint}</TableHead>
                <TableHead class="w-[10%]">{$messages.docker.volumes.scope}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} class="w-[15%]" />
                <TableHead class="w-[10%]">{$messages.common.actions}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each sortedVolumes as volume}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                        <a
                          href="/nodes/{data.nodeId}/docker/volumes/{encodeURIComponent(volume.name)}"
                          class="font-medium truncate max-w-[180px] block hover:underline"
                          title={volume.name}
                        >
                          {volume.name}
                        </a>
                      {#if volume.labels && Object.keys(volume.labels).length > 0}
                        <div class="flex flex-wrap gap-1">
                          {#each Object.entries(volume.labels).slice(0, 2) as [key, value]}
                            <span class="text-xs text-muted-foreground bg-muted/50 px-1 rounded" title="{key}={value}">
                              {key}
                            </span>
                          {/each}
                          {#if Object.keys(volume.labels).length > 2}
                            <span class="text-xs text-muted-foreground">+{Object.keys(volume.labels).length - 2}</span>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">{volume.driver}</Badge>
                  </TableCell>
                  <TableCell>
                    <span class="text-sm">{formatBytes(volume.sizeBytes)}</span>
                  </TableCell>
                  <TableCell>
                    {#if volume.inUse}
                      <Badge variant="default">{volume.containersCount} {$messages.docker.containers.title}</Badge>
                    {:else}
                      <Badge variant="secondary">{$messages.docker.volumes.unused}</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-1">
                      <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded truncate max-w-[200px]" title={volume.mountpoint}>
                        {volume.mountpoint}
                      </code>
                      <CopyButton text={volume.mountpoint} label={$messages.common.copy} />
                    </div>
                  </TableCell>
                  <TableCell>
                    <span class="text-sm">{volume.scope}</span>
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={volume.created}>
                      {formatDockerTimestamp(volume.created)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="destructive"
                      size="sm"
                      onclick={() => openRemoveDialog(volume)}
                      disabled={removeBusyId === volume.name}
                    >
                      {$messages.common.delete}
                    </Button>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredVolumes.length !== volumes.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              {$messages.docker.volumes.countSummary.replace('{shown}', String(filteredVolumes.length)).replace('{total}', String(volumes.length))}
            </div>
          {/if}
        {:else if searchQuery}
          <div class="empty-state">
            {$messages.common.noData}
          </div>
        {:else}
          <div class="empty-state">{$messages.docker.volumes.noVolumes}</div>
        {/if}
      </CardContent>
    </Card>

    <Dialog bind:open={removeDialogOpen}>
      <DialogOverlay />
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{$messages.docker.volumes.removeDialogTitle}</DialogTitle>
          <DialogDescription>{removeDescription}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onclick={() => (removeDialogOpen = false)}>
            {$messages.common.cancel}
          </Button>
          <Button type="button" variant="destructive" onclick={() => void queueVolumeRemove()} disabled={!removeTarget || removeBusyId === removeTarget.name}>
            {$messages.common.delete}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
