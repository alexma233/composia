<script lang="ts">
  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="page-shell">
  <Card class="border-border/70 bg-card/95">
    <CardHeader class="gap-4">
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1">
          <CardTitle class="page-title">Backup history</CardTitle>
          <CardDescription class="page-description">Controller-recorded backup runs.</CardDescription>
        </div>
        <Badge variant="outline">{data.backups.length}</Badge>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>Load failed</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
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
        <div class="empty-state">No backups loaded.</div>
      {/if}
    </CardContent>
  </Card>
</div>
