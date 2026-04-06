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
          <CardTitle class="page-title">Task history</CardTitle>
          <CardDescription class="page-description">Recent queue activity.</CardDescription>
        </div>
        <Badge variant="outline">{data.tasks.length}</Badge>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>Load failed</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
      {#if data.tasks.length}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Task</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Scope</TableHead>
              <TableHead class="w-56">Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {#each data.tasks as task}
              <TableRow>
                <TableCell>
                  <a href={`/tasks/${task.taskId}`} class="font-medium hover:text-primary">{task.type}</a>
                  <div class="text-xs text-muted-foreground">{task.taskId}</div>
                </TableCell>
                <TableCell>
                  <Badge variant={taskStatusTone(task.status)}>{task.status}</Badge>
                </TableCell>
                <TableCell class="text-muted-foreground">
                  {task.serviceName ? task.serviceName : `node: ${task.nodeId || 'n/a'}`}
                </TableCell>
                <TableCell class="text-muted-foreground">{formatTimestamp(task.createdAt)}</TableCell>
              </TableRow>
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">No tasks loaded.</div>
      {/if}
    </CardContent>
  </Card>
</div>
