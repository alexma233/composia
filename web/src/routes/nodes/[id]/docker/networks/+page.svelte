<script lang="ts">
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type Variant } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';

  export let data: PageData;

  let searchQuery = '';
  let sortField: 'name' | 'driver' | 'created' = 'name';
  let sortDirection: 'asc' | 'desc' = 'asc';

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

  $: filteredNetworks = (data.networks || []).filter((n) => {
    const query = searchQuery.toLowerCase();
    return (
      n.name.toLowerCase().includes(query) ||
      n.driver.toLowerCase().includes(query) ||
      n.id.toLowerCase().includes(query) ||
      n.scope.toLowerCase().includes(query)
    );
  });

  $: sortedNetworks = [...filteredNetworks].sort((a, b) => {
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
  });
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
              {#if data.networks}
                <Badge variant="secondary" class="ml-2">{data.networks.length}</Badge>
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
            <Button variant="ghost" size="sm" on:click={() => (searchQuery = '')}>
              Clear
            </Button>
          {/if}
        </div>
      </CardHeader>
      <CardContent>
        {#if data.error}
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            {data.error}
          </div>
        {:else if sortedNetworks.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="w-[25%]">
                  <button class="flex items-center gap-1 hover:text-foreground" on:click={() => handleSort('name')}>
                    Name
                    {@html SortIcon('name')}
                  </button>
                </TableHead>
                <TableHead class="w-[10%]">
                  <button class="flex items-center gap-1 hover:text-foreground" on:click={() => handleSort('driver')}>
                    Driver
                    {@html SortIcon('driver')}
                  </button>
                </TableHead>
                <TableHead class="w-[10%]">Scope</TableHead>
                <TableHead class="w-[15%]">Subnet</TableHead>
                <TableHead class="w-[10%]">Containers</TableHead>
                <TableHead class="w-[10%]">Attributes</TableHead>
                <TableHead class="w-[15%]">
                  <button class="flex items-center gap-1 hover:text-foreground" on:click={() => handleSort('created')}>
                    Created
                    {@html SortIcon('created')}
                  </button>
                </TableHead>
                <TableHead class="w-[5%] text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each sortedNetworks as network}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                      <div class="flex items-center gap-2">
                        <span class="font-medium">{network.name}</span>
                        {#if isSystemNetwork(network.name)}
                          <Badge variant="secondary" class="text-xs">System</Badge>
                        {/if}
                      </div>
                      <div class="flex items-center gap-1.5">
                        <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded">
                          {formatShortId(network.id)}
                        </code>
                        <button
                          on:click={() => copyToClipboard(network.id)}
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
                    <div class="flex flex-wrap gap-1">
                      {#if network.internal}
                        <Badge variant="warning" class="text-xs">Internal</Badge>
                      {/if}
                      {#if network.attachable}
                        <Badge variant="info" class="text-xs">Attachable</Badge>
                      {/if}
                      {#if network.ipv6Enabled}
                        <Badge variant="outline" class="text-xs">IPv6</Badge>
                      {/if}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={network.created}>
                      {formatRelativeTime(network.created)}
                    </div>
                  </TableCell>
                  <TableCell class="text-right">
                    <a
                      href="/nodes/{data.nodeId}/docker/networks/{encodeURIComponent(network.id)}"
                      class="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-8 px-3"
                    >
                      Inspect
                    </a>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredNetworks.length !== data.networks?.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredNetworks.length} of {data.networks?.length} networks
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