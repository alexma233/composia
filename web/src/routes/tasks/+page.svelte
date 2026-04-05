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
        <h1 class="text-2xl font-semibold">Tasks</h1>
        <p class="text-sm text-muted-foreground">Recent task history from the durable queue.</p>
      </div>
      <span class="rounded-md border bg-muted/50 px-2.5 py-1 text-xs text-muted-foreground">
        {data.tasks.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
        {data.error}
      </div>
    {/if}

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
                {task.serviceName || 'system task'} on {task.nodeId || 'n/a'}
              </TableCell>
              <TableCell class="text-muted-foreground">{formatTimestamp(task.createdAt)}</TableCell>
            </TableRow>
          {/each}
        </TableBody>
      </Table>
    {:else}
      <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
        No tasks loaded.
      </div>
    {/if}
  </div>
</div>
