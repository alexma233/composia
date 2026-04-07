<script lang="ts">
  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, onlineStatusTone } from '$lib/presenters';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();
</script>

<div class="page-shell">
	<Card>
		<CardHeader>
      <div class="flex items-start justify-between gap-4">
        <CardTitle class="page-title">Nodes</CardTitle>
        <Badge variant="outline">{data.nodes.length}</Badge>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>Load failed</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
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
        <div class="empty-state">No nodes loaded.</div>
      {/if}
    </CardContent>
  </Card>
</div>
