<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type Variant } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import { formatDockerTimestamp, formatShortId } from '$lib/presenters';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  type DockerNetworkSummary = {
    id: string;
    name: string;
    driver: string;
    scope: string;
    internal: boolean;
    attachable: boolean;
    created: string;
    labels: Record<string, string>;
    subnet: string;
    gateway: string;
    containersCount: number;
    ipv6Enabled: boolean;
  };

  let searchQuery = $state('');
  let sortField = $state<'name' | 'driver' | 'created'>('name');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let networks = $state<DockerNetworkSummary[]>([]);

  $effect(() => {
    loading = !data.ready;
    loadError = data.error ?? null;
    networks = data.networks || [];
  });

  async function loadNetworks() {
    if (!data.ready) {
      loading = false;
      return;
    }

    loading = true;
    loadError = null;

    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/networks/data`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error || 'Failed to load networks');
      }
      networks = payload.networks ?? [];
    } catch (error) {
      loadError = error instanceof Error ? error.message : 'Failed to load networks';
      networks = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void loadNetworks();
  });

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function isSystemNetwork(name: string): boolean {
    return name === 'bridge' || name === 'host' || name === 'none';
  }

  function getDriverVariant(driver: string): Variant {
    const d = driver.toLowerCase();
    if (d === 'bridge' || d === 'host') return 'default';
    if (d === 'overlay') return 'info';
    if (d === 'macvlan') return 'secondary';
    return 'outline';
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

  let filteredNetworks = $derived(networks.filter((n) => {
    const query = searchQuery.toLowerCase();
    return (
      n.name.toLowerCase().includes(query) ||
      n.driver.toLowerCase().includes(query) ||
      n.id.toLowerCase().includes(query) ||
      n.scope.toLowerCase().includes(query)
    );
  }));

  let sortedNetworks = $derived([...filteredNetworks].sort((a, b) => {
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
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">Networks</CardTitle>
            <CardDescription class="page-description">
              Docker networks on {data.nodeId}
              {#if !loading}
                <Badge variant="secondary" class="ml-2">{networks.length}</Badge>
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
              placeholder="Search networks..."
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
              Clear
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void loadNetworks()} disabled={loading || !data.ready}>
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
              <span>Loading networks...</span>
            </div>
          </div>
        {:else if sortedNetworks.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="w-[30%]">
                  <button class="flex items-center gap-1 hover:text-foreground" onclick={() => handleSort('name')}>
                    Name
                    {@html SortIcon('name')}
                  </button>
                </TableHead>
                <TableHead class="w-[10%]">
                  <button class="flex items-center gap-1 hover:text-foreground" onclick={() => handleSort('driver')}>
                    Driver
                    {@html SortIcon('driver')}
                  </button>
                </TableHead>
                <TableHead class="w-[10%]">Scope</TableHead>
                <TableHead class="w-[15%]">Subnet</TableHead>
                <TableHead class="w-[10%]">Containers</TableHead>
                <TableHead class="w-[20%]">
                  <button class="flex items-center gap-1 hover:text-foreground" onclick={() => handleSort('created')}>
                    Created
                    {@html SortIcon('created')}
                  </button>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each sortedNetworks as network}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                      <div class="flex items-center gap-2">
                        <a
                          href="/nodes/{data.nodeId}/docker/networks/{encodeURIComponent(network.id)}"
                          class="font-medium hover:underline"
                        >
                          {network.name}
                        </a>
                        {#if isSystemNetwork(network.name)}
                          <Badge variant="secondary" class="text-xs">System</Badge>
                        {/if}
                      </div>
                      <div class="flex items-center gap-1.5">
                        <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded">
                          {formatShortId(network.id)}
                        </code>
                        <button
                          onclick={() => copyToClipboard(network.id)}
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
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={getDriverVariant(network.driver)}>{network.driver}</Badge>
                  </TableCell>
                  <TableCell>
                    <span class="text-sm">{network.scope}</span>
                  </TableCell>
                  <TableCell>
                    {#if network.subnet}
                      <code class="text-xs bg-muted px-1 py-0.5 rounded">{network.subnet}</code>
                    {:else}
                      <span class="text-muted-foreground">-</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    {#if network.containersCount > 0}
                      <Badge variant="success">{network.containersCount}</Badge>
                    {:else}
                      <span class="text-muted-foreground">0</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={network.created}>
                      {formatDockerTimestamp(network.created)}
                    </div>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredNetworks.length !== networks.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredNetworks.length} of {networks.length} networks
            </div>
          {/if}
        {:else if searchQuery}
          <div class="empty-state">
            No networks matching "{searchQuery}".
          </div>
        {:else}
          <div class="empty-state">No networks found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
