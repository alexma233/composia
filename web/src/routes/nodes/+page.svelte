<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, onlineStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-lg border bg-card p-6 shadow-xs">
    <div class="mb-6 flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Nodes</h1>
        <p class="text-sm text-muted-foreground">Configured nodes and live heartbeat state.</p>
      </div>
      <span class="rounded-md border bg-muted/50 px-2.5 py-1 text-xs text-muted-foreground">
        {data.nodes.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
        {data.error}
      </div>
    {/if}

    {#if data.nodes.length}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Node</TableHead>
            <TableHead>Status</TableHead>
            <TableHead class="w-56">Last heartbeat</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {#each data.nodes as node}
            <TableRow>
              <TableCell>
                <a href={`/nodes/${node.nodeId}`} class="font-medium hover:text-primary">{node.displayName}</a>
                <div class="text-xs text-muted-foreground">{node.nodeId}</div>
              </TableCell>
              <TableCell>
                <Badge variant={onlineStatusTone(node.isOnline)}>
                  {node.isOnline ? 'online' : 'offline'}
                </Badge>
              </TableCell>
              <TableCell class="text-muted-foreground">{formatTimestamp(node.lastHeartbeat)}</TableCell>
            </TableRow>
          {/each}
        </TableBody>
      </Table>
    {:else}
      <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
        No nodes loaded.
      </div>
    {/if}
  </div>
</div>
