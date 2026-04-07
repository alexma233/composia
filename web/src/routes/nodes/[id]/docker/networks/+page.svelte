<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type Variant } from '$lib/components/ui/badge';
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

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as typeof sortField;
      sortDirection = 'asc';
    }
  }

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
            <SearchIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
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
          <Alert variant="destructive">
            <AlertDescription>{loadError}</AlertDescription>
          </Alert>
        {:else if loading}
          <div class="flex min-h-[320px] items-center justify-center">
            <div class="flex items-center gap-3 text-sm text-muted-foreground">
              <Spinner />
              <span>Loading networks...</span>
            </div>
          </div>
        {:else if sortedNetworks.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label="Name" {sortField} {sortDirection} onSort={handleSort} class="w-[30%]" />
                <SortableTableHead field="driver" label="Driver" {sortField} {sortDirection} onSort={handleSort} class="w-[10%]" />
                <TableHead class="w-[10%]">Scope</TableHead>
                <TableHead class="w-[15%]">Subnet</TableHead>
                <TableHead class="w-[10%]">Containers</TableHead>
                <SortableTableHead field="created" label="Created" {sortField} {sortDirection} onSort={handleSort} class="w-[20%]" />
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
                        <CopyButton text={network.id} label="Copy full ID" />
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
