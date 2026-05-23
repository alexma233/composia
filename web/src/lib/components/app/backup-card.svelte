<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { messages } from '$lib/i18n';
  import { formatTimestamp, taskStatusLabel, taskStatusTone } from '$lib/presenters';
  import type { BackupSummary } from '$lib/server/controller';

  interface Props {
    backup: BackupSummary;
  }

  let { backup }: Props = $props();
</script>

<a href={`/backups/${backup.backupId}`} class="list-row">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-medium">{backup.dataName}</div>
      <div class="truncate text-xs text-muted-foreground">{backup.backupId}</div>
    </div>
    <Badge variant={taskStatusTone(backup.status)}>{taskStatusLabel(backup.status, $messages)}</Badge>
  </div>
  <div class="mt-2 text-xs text-muted-foreground">
    {formatTimestamp(backup.finishedAt || backup.startedAt)}
  </div>
</a>
