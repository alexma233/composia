<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { formatTimestamp, runtimeStatusTone, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-lg border bg-card p-6 shadow-xs">
      {#if data.service}
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div class="text-sm text-muted-foreground">Service detail</div>
            <h1 class="mt-1 text-3xl font-semibold tracking-tight">{data.service.name}</h1>
            <div class="mt-2 text-sm text-muted-foreground">
              Node {data.service.node} · updated {formatTimestamp(data.service.updatedAt)}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <a href={`/services/${data.service.name}/secret`} class="inline-flex h-9 items-center rounded-md border bg-background px-4 text-sm transition-colors hover:bg-muted/40">Edit secret</a>
            <Badge variant={runtimeStatusTone(data.service.runtimeStatus)}>
              {data.service.runtimeStatus}
            </Badge>
          </div>
        </div>
      {:else}
        <h1 class="text-3xl font-semibold tracking-tight">Service detail</h1>
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="rounded-lg border bg-card p-6 shadow-xs">
      <Tabs value="tasks">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-xl font-medium">Activity</h2>
            <p class="text-sm text-muted-foreground">Recent tasks and backups for this service.</p>
          </div>
          <TabsList>
            <TabsTrigger value="tasks">Tasks</TabsTrigger>
            <TabsTrigger value="backups">Backups</TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="tasks">
          {#if data.tasks.length}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Task</TableHead>
                  <TableHead>Status</TableHead>
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
                    <TableCell class="text-muted-foreground">{formatTimestamp(task.createdAt)}</TableCell>
                  </TableRow>
                {/each}
              </TableBody>
            </Table>
          {:else}
            <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No tasks loaded.</div>
          {/if}
        </TabsContent>

        <TabsContent value="backups">
          {#if data.backups.length}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Data</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead class="w-56">Finished</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {#each data.backups as backup}
                  <TableRow>
                    <TableCell>
                      <div class="font-medium">{backup.dataName}</div>
                      <div class="text-xs text-muted-foreground">{backup.backupId}</div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={taskStatusTone(backup.status)}>{backup.status}</Badge>
                    </TableCell>
                    <TableCell class="text-muted-foreground">{formatTimestamp(backup.finishedAt || backup.startedAt)}</TableCell>
                  </TableRow>
                {/each}
              </TableBody>
            </Table>
          {:else}
            <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No backups loaded.</div>
          {/if}
        </TabsContent>
      </Tabs>
    </section>
  </div>
</div>
