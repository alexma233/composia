<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-lg border bg-card p-6 shadow-xs">
    <div class="mb-6 flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Backups</h1>
        <p class="text-sm text-muted-foreground">Global backup history recorded by the controller.</p>
      </div>
      <span class="rounded-md border bg-muted/50 px-2.5 py-1 text-xs text-muted-foreground">
        {data.backups.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">{data.error}</div>
    {/if}

    {#if data.backups.length}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Backup</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Task</TableHead>
            <TableHead class="w-56">Finished</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {#each data.backups as backup}
            <TableRow>
              <TableCell>
                <div class="font-medium">{backup.serviceName} / {backup.dataName}</div>
                <div class="text-xs text-muted-foreground">{backup.backupId}</div>
              </TableCell>
              <TableCell>
                <Badge variant={taskStatusTone(backup.status)}>{backup.status}</Badge>
              </TableCell>
              <TableCell class="text-muted-foreground">{backup.taskId}</TableCell>
              <TableCell class="text-muted-foreground">{formatTimestamp(backup.finishedAt || backup.startedAt)}</TableCell>
            </TableRow>
          {/each}
        </TableBody>
      </Table>
    {:else}
      <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No backups loaded.</div>
    {/if}
  </div>
</div>
