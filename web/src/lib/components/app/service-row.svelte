<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { TableCell, TableRow } from '$lib/components/ui/table';
  import { messages } from '$lib/i18n';
  import { formatTimestamp, runtimeStatusTone } from '$lib/presenters';
  import type { ServiceWorkspaceSummary } from '$lib/server/controller';

  interface Props {
    service: ServiceWorkspaceSummary;
  }

  let { service }: Props = $props();

  function statusTone(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) return 'outline';
    if (runtimeStatus === 'needs_validation') return 'secondary';
    return runtimeStatusTone(runtimeStatus || 'unknown');
  }

  function statusText(hasMeta: boolean, runtimeStatus: string) {
    if (!hasMeta) return $messages.services.noMeta;
    if (runtimeStatus === 'needs_validation') return $messages.services.metaDraft;
    return runtimeStatus || $messages.common.unknown;
  }
</script>

<TableRow class="hover:bg-accent/50">
  <TableCell>
    <a href={`/services/${service.folder}`} class="font-medium hover:text-primary">{service.displayName}</a>
  </TableCell>
  <TableCell class="text-muted-foreground">{service.folder}</TableCell>
  <TableCell class="max-w-64 truncate text-muted-foreground">
    {service.nodes.length ? service.nodes.join(", ") : $messages.common.na}
  </TableCell>
  <TableCell>
    <Badge variant={statusTone(service.hasMeta, service.runtimeStatus)}>
      {statusText(service.hasMeta, service.runtimeStatus)}
    </Badge>
  </TableCell>
  <TableCell class="text-muted-foreground">
    {#if service.updatedAt}
      {formatTimestamp(service.updatedAt)}
    {:else if service.hasMeta}
      {$messages.services.metaExistsNotDeclared}
    {:else}
      {$messages.services.noMetaFile}
    {/if}
  </TableCell>
</TableRow>
