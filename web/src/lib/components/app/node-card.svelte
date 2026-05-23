<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { messages } from '$lib/i18n';
  import { formatTimestamp, onlineStatusTone } from '$lib/presenters';
  import type { NodeSummary } from '$lib/server/controller';

  interface Props {
    node: NodeSummary;
  }

  let { node }: Props = $props();
</script>

<a href={`/nodes/${node.nodeId}`} class="list-row">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-medium">{node.displayName}</div>
      <div class="truncate text-xs text-muted-foreground">{node.nodeId}</div>
    </div>
    <Badge variant={onlineStatusTone(node.isOnline)}>
      {node.isOnline ? $messages.status.online : $messages.status.offline}
    </Badge>
  </div>
  <div class="mt-2 text-xs text-muted-foreground">
    {$messages.dashboard.lastHeartbeat} {formatTimestamp(node.lastHeartbeat)}
  </div>
</a>
