<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import { formatDockerTimestamp, formatShortId } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
  import { Alert, AlertDescription } from '$lib/components/ui/alert';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  type DockerContainerSummary = {
    id: string;
    name: string;
    image: string;
    state: string;
    status: string;
    created: string;
    labels: Record<string, string>;
    ports: string[];
    networks: string[];
    imageId: string;
  };

  let searchQuery = $state('');
  let sortField = $state<'name' | 'state' | 'image' | 'created'>('name');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let containers = $state<DockerContainerSummary[]>([]);
  let actionBusyId = $state('');

  $effect(() => {
    loading = !data.ready;
    loadError = data.error ?? null;
    containers = data.containers || [];
  });

  async function loadContainers() {
    if (!data.ready) {
      loading = false;
      return;
    }

    loading = true;
    loadError = null;

    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/data`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error || 'Failed to load containers');
      }
      containers = payload.containers ?? [];
    } catch (error) {
      loadError = error instanceof Error ? error.message : 'Failed to load containers';
      containers = [];
    } finally {
      loading = false;
    }
  }

  async function runAction(containerId: string, action: 'start' | 'stop' | 'restart') {
    actionBusyId = `${containerId}:${action}`;
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action, containerId })
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? `Failed to ${action} container`);
      }
      toast.success(`${action} queued: ${payload.taskId?.slice(0, 12) ?? 'task'}`);
      await loadContainers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : `Failed to ${action} container.`);
    } finally {
      actionBusyId = '';
    }
  }

  function isActionBusy(containerId: string, action: 'start' | 'stop' | 'restart') {
    return actionBusyId === `${containerId}:${action}`;
  }

  onMount(() => {
    void loadContainers();
  });

  function getStateVariant(state: string): BadgeVariant {
    const s = (state || '').toLowerCase();
    if (s === 'running') return 'default';
    if (s === 'created' || s === 'starting') return 'outline';
    if (s === 'paused') return 'secondary';
    if (s === 'restarting' || s === 'unhealthy') return 'outline';
    if (s === 'exited' || s === 'dead' || s === 'removing') return 'destructive';
    return 'default';
  }

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as typeof sortField;
      sortDirection = 'asc';
    }
  }

  let filteredContainers = $derived(containers.filter((c) => {
    const query = searchQuery.toLowerCase();
    return (
      c.name.toLowerCase().includes(query) ||
      c.image.toLowerCase().includes(query) ||
      c.state.toLowerCase().includes(query) ||
      c.id.toLowerCase().includes(query) ||
      (c.networks || []).some((n) => n.toLowerCase().includes(query))
    );
  }));

  let sortedContainers = $derived([...filteredContainers].sort((a, b) => {
    let comparison = 0;
    switch (sortField) {
      case 'name':
        comparison = a.name.localeCompare(b.name);
        break;
      case 'state':
        comparison = a.state.localeCompare(b.state);
        break;
      case 'image':
        comparison = a.image.localeCompare(b.image);
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
            <CardTitle class="page-title">Containers</CardTitle>
            <CardDescription class="page-description">
              Docker containers on {data.nodeId}
              {#if !loading}
                <Badge variant="secondary" class="ml-2">{containers.length}</Badge>
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
              placeholder="Search containers..."
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
              Clear
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void loadContainers()} disabled={loading || !data.ready}>
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
              <span>Loading containers...</span>
            </div>
          </div>
        {:else if sortedContainers.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label="Name" {sortField} {sortDirection} onSort={handleSort} class="w-[30%]" />
                <SortableTableHead field="state" label="State" {sortField} {sortDirection} onSort={handleSort} class="w-[10%]" />
                <SortableTableHead field="image" label="Image" {sortField} {sortDirection} onSort={handleSort} class="w-[20%]" />
                <TableHead class="w-[15%]">Ports</TableHead>
                <TableHead class="w-[15%]">Networks</TableHead>
                <SortableTableHead field="created" label="Created" {sortField} {sortDirection} onSort={handleSort} class="w-[12%]" />
                <TableHead class="w-[18%]">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each sortedContainers as container}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                        <a
                          href="/nodes/{data.nodeId}/docker/containers/{encodeURIComponent(container.id)}"
                          class="font-medium hover:underline"
                        >
                          {container.name}
                        </a>
                      <div class="flex items-center gap-1.5">
                        <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded">
                          {formatShortId(container.id)}
                        </code>
                        <CopyButton text={container.id} label="Copy full ID" />
                      </div>
                      {#if container.labels && Object.keys(container.labels).length > 0}
                        <div class="flex flex-wrap gap-1">
                          {#each Object.entries(container.labels).slice(0, 2) as [key, value]}
                            <span class="text-xs text-muted-foreground bg-muted/50 px-1 rounded" title="{key}={value}">
                              {key}
                            </span>
                          {/each}
                          {#if Object.keys(container.labels).length > 2}
                            <span class="text-xs text-muted-foreground">+{Object.keys(container.labels).length - 2}</span>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={getStateVariant(container.state)}>
                      {container.state}
                    </Badge>
                    {#if container.status}
                      <div class="text-xs text-muted-foreground mt-1">{container.status}</div>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="truncate max-w-[200px]" title={container.image}>
                      {container.image}
                    </div>
                  </TableCell>
                  <TableCell>
                    {#if container.ports && container.ports.length > 0}
                      <div class="space-y-0.5">
                        {#each container.ports.slice(0, 3) as port}
                          <code class="text-xs bg-muted px-1 py-0.5 rounded block truncate">{port}</code>
                        {/each}
                        {#if container.ports.length > 3}
                          <span class="text-xs text-muted-foreground">+{container.ports.length - 3} more</span>
                        {/if}
                      </div>
                    {:else}
                      <span class="text-muted-foreground">-</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    {#if container.networks && container.networks.length > 0}
                      <div class="flex flex-wrap gap-1">
                        {#each container.networks.slice(0, 2) as network}
                          <Badge variant="outline" class="text-xs">{network}</Badge>
                        {/each}
                        {#if container.networks.length > 2}
                          <span class="text-xs text-muted-foreground">+{container.networks.length - 2}</span>
                        {/if}
                      </div>
                    {:else}
                      <span class="text-muted-foreground">-</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={container.created}>
                      {formatDockerTimestamp(container.created)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div class="flex flex-wrap gap-2">
                      <a href="/nodes/{data.nodeId}/docker/containers/{encodeURIComponent(container.id)}?tab=logs">
                        <Button variant="outline" size="sm">Logs</Button>
                      </a>
                      <a href="/nodes/{data.nodeId}/docker/containers/{encodeURIComponent(container.id)}?tab=terminal">
                        <Button variant="outline" size="sm">Terminal</Button>
                      </a>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void runAction(container.id, 'start')}
                        disabled={isActionBusy(container.id, 'start') || container.state.toLowerCase() === 'running'}
                      >
                        Start
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void runAction(container.id, 'stop')}
                        disabled={isActionBusy(container.id, 'stop') || container.state.toLowerCase() !== 'running'}
                      >
                        Stop
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void runAction(container.id, 'restart')}
                        disabled={isActionBusy(container.id, 'restart')}
                      >
                        Restart
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredContainers.length !== containers.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredContainers.length} of {containers.length} containers
            </div>
          {/if}
        {:else if searchQuery}
          <div class="empty-state">
            No containers matching "{searchQuery}".
          </div>
        {:else}
          <div class="empty-state">No containers found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
