<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { TableCell, TableRow } from '$lib/components/ui/table';
  import { getMessages } from '$lib/i18n';

  const messages = getMessages();
  import { formatTimestamp, taskStatusLabel, taskStatusTone } from '$lib/presenters';
  import type { BackupSummary } from '$lib/server/controller';
  interface Props {
    backup: BackupSummary;
  }

  let { backup }: Props = $props();
</script>

<TableRow class="hover:bg-accent/50">
  <TableCell>
    <a href={`/backups/${backup.backupId}`} class="font-medium hover:text-primary">{backup.serviceName} / {backup.dataName}</a>
    <div class="text-xs text-muted-foreground">{backup.backupId}</div>
  </TableCell>
  <TableCell>
    <Badge variant={taskStatusTone(backup.status)}>{taskStatusLabel(backup.status, $messages)}</Badge>
  </TableCell>
  <TableCell class="text-muted-foreground">{backup.taskId}</TableCell>
  <TableCell class="text-muted-foreground">{formatTimestamp(backup.finishedAt || backup.startedAt)}</TableCell>
</TableRow>
