<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { messages } from '$lib/i18n';
  import { formatTimestamp, runtimeStatusLabel, runtimeStatusTone } from '$lib/presenters';
  import type { ServiceSummary } from '$lib/server/controller';

  interface Props {
    service: ServiceSummary;
  }

  let { service }: Props = $props();
</script>

<a href={`/services/${service.folder ?? service.name}`} class="list-row">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-medium">{service.name}</div>
      <div class="truncate text-xs text-muted-foreground">
        {$messages.dashboard.updated} {formatTimestamp(service.updatedAt)}
      </div>
    </div>
    <Badge variant={runtimeStatusTone(service.runtimeStatus)}>
      {runtimeStatusLabel(service.runtimeStatus, $messages)}
    </Badge>
  </div>
</a>
