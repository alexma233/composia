<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { TableCell, TableRow } from '$lib/components/ui/table';
  import { messages } from '$lib/i18n';
  import { formatTimestamp, onlineStatusTone } from '$lib/presenters';
  import type { NodeSummary } from '$lib/server/controller';

  interface Props {
    node: NodeSummary;
  }

  let { node }: Props = $props();
</script>

<TableRow class="hover:bg-accent/50">
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
