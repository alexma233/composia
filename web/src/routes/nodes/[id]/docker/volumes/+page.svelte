<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import { formatBytes, formatDockerTimestamp } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
  import { Alert, AlertDescription } from '$lib/components/ui/alert';

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
        throw new Error(payload.error || 'Failed to load volumes');
      }
      volumes = payload.volumes ?? [];
    } catch (error) {
      loadError = error instanceof Error ? error.message : 'Failed to load volumes';
      volumes = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void refreshVolumes();
  });

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
            <CardTitle class="page-title">Volumes</CardTitle>
            <CardDescription class="page-description">
              Docker volumes on {data.nodeId}
              {#if !loading}
                <Badge variant="secondary" class="ml-2">{volumes.length}</Badge>
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            Back to node
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <SearchIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search volumes..."
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
              Clear
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void refreshVolumes()} disabled={loading || !data.ready}>
            {#if loading}Loading...{:else}Refresh{/if}
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
              <span>Loading volumes and usage data...</span>
            </div>
          </div>
        {:else if sortedVolumes.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label="Name" {sortField} {sortDirection} onSort={handleSort} class="w-[25%]" />
                <TableHead class="w-[10%]">Driver</TableHead>
                <TableHead class="w-[10%]">Size</TableHead>
                <TableHead class="w-[10%]">Usage</TableHead>
                <TableHead class="w-[25%]">Mount Point</TableHead>
                <TableHead class="w-[10%]">Scope</TableHead>
                <SortableTableHead field="created" label="Created" {sortField} {sortDirection} onSort={handleSort} class="w-[15%]" />
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
                      <Badge variant="default">{volume.containersCount} container{volume.containersCount > 1 ? 's' : ''}</Badge>
                    {:else}
                      <Badge variant="secondary">Unused</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-1">
                      <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded truncate max-w-[200px]" title={volume.mountpoint}>
                        {volume.mountpoint}
                      </code>
                      <CopyButton text={volume.mountpoint} label="Copy mount point" />
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
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredVolumes.length !== volumes.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredVolumes.length} of {volumes.length} volumes
            </div>
          {/if}
        {:else if searchQuery}
          <div class="empty-state">
            No volumes matching "{searchQuery}".
          </div>
        {:else}
          <div class="empty-state">No volumes found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
