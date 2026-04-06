<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type Variant } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';

  let { data }: { data: PageData } = $props();

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
  let loading = $state(data.ready);
  let loadError = $state(data.error);
  let containers = $state<DockerContainerSummary[]>(data.containers || []);

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

  onMount(() => {
    void loadContainers();
  });

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function formatShortId(id: string): string {
    return id.substring(0, 12);
  }

  function formatRelativeTime(timestamp: string): string {
    if (!timestamp) return '-';
    
    // Handle Docker's "2006-01-02 15:04:05 +0700 MST" format
    let date: Date;
    if (timestamp.includes(' +') || timestamp.includes(' -')) {
      // Remove timezone suffix like " +0800 CST" and parse
      const cleaned = timestamp.replace(/\s+[+-]\d{4}\s+\w+$/, '');
      const parts = cleaned.split(' ');
      if (parts.length === 2) {
        date = new Date(parts[0] + 'T' + parts[1]);
      } else {
        date = new Date(cleaned);
      }
    } else {
      date = new Date(timestamp);
    }
    
    if (isNaN(date.getTime())) return timestamp;
    const now = new Date();
    const diff = Math.floor((now.getTime() - date.getTime()) / 1000);

    if (diff < 0) return 'just now';
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  }

  function getStateVariant(state: string): Variant {
    const s = (state || '').toLowerCase();
    if (s === 'running') return 'success';
    if (s === 'created' || s === 'starting') return 'info';
    if (s === 'paused') return 'secondary';
    if (s === 'restarting' || s === 'unhealthy') return 'warning';
    if (s === 'exited' || s === 'dead' || s === 'removing') return 'danger';
    return 'default';
  }

  function handleSort(field: typeof sortField) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field;
      sortDirection = 'asc';
    }
  }

  const SortIcon = (field: typeof sortField) => {
    if (sortField !== field) {
      return `<svg class="w-3 h-3 text-muted-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M7 15l5 5 5-5M7 9l5-5 5 5"/>
      </svg>`;
    }
    if (sortDirection === 'asc') {
      return `<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 5v14M5 12l7-7 7 7"/>
      </svg>`;
    }
    return `<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M12 19V5M5 12l7 7 7-7"/>
    </svg>`;
  };

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
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
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
            <svg
              class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.3-4.3" />
            </svg>
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
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            {loadError}
          </div>
        {:else if loading}
          <div class="flex min-h-[320px] items-center justify-center">
            <div class="flex items-center gap-3 text-sm text-muted-foreground">
              <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4H4z"></path>
              </svg>
              <span>Loading containers...</span>
            </div>
          </div>
        {:else if sortedContainers.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="w-[30%]">
                  <button
                    class="flex items-center gap-1 hover:text-foreground"
                    onclick={() => handleSort('name')}
                  >
                    Name
                    {@html SortIcon('name')}
                  </button>
                </TableHead>
                <TableHead class="w-[10%]">
                  <button
                    class="flex items-center gap-1 hover:text-foreground"
                    onclick={() => handleSort('state')}
                  >
                    State
                    {@html SortIcon('state')}
                  </button>
                </TableHead>
                <TableHead class="w-[20%]">
                  <button
                    class="flex items-center gap-1 hover:text-foreground"
                    onclick={() => handleSort('image')}
                  >
                    Image
                    {@html SortIcon('image')}
                  </button>
                </TableHead>
                <TableHead class="w-[15%]">Ports</TableHead>
                <TableHead class="w-[15%]">Networks</TableHead>
                <TableHead class="w-[15%]">
                  <button
                    class="flex items-center gap-1 hover:text-foreground"
                    onclick={() => handleSort('created')}
                  >
                    Created
                    {@html SortIcon('created')}
                  </button>
                </TableHead>
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
<button
                           onclick={() => copyToClipboard(container.id)}
                           class="text-muted-foreground hover:text-foreground"
                           title="Copy full ID"
                         >
                          <svg
                            class="h-3.5 w-3.5"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          >
                            <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
                            <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
                          </svg>
                        </button>
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
                      {formatRelativeTime(container.created)}
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
