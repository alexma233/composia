<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { onMount } from 'svelte';
  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { startPolling } from '$lib/refresh';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, onlineStatusTone } from '$lib/presenters';
  import { messages } from '$lib/i18n';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  onMount(() => startPolling(() => invalidateAll(), { intervalMs: 5000 }));
</script>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title">{$messages.nodes.title}</CardTitle>
        </div>
        <Badge variant="outline">{data.nodes.length}</Badge>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
      {#if data.nodes.length}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{$messages.nodes.node}</TableHead>
              <TableHead>{$messages.common.status}</TableHead>
              <TableHead class="w-56">{$messages.nodes.lastHeartbeat}</TableHead>
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
                    {node.isOnline ? $messages.status.online : $messages.status.offline}
                  </Badge>
                </TableCell>
                <TableCell class="text-muted-foreground">{formatTimestamp(node.lastHeartbeat)}</TableCell>
              </TableRow>
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">{$messages.nodes.noNodes}</div>
      {/if}
    </CardContent>
  </Card>
</div>
