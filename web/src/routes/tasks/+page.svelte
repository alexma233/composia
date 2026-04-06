<script lang="ts">
  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { taskStatusTone } from '$lib/presenters';
  import TaskItem from '$lib/components/app/task-item.svelte';

  export let data: PageData;
</script>

<div class="page-shell">
  <Card class="border-border/70 bg-card/95">
    <CardHeader class="gap-4">
      <div class="flex items-start justify-between gap-4">
        <CardTitle class="page-title">Task history</CardTitle>
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
      <div class="space-y-3">
        {#if data.tasks.length}
          {#each data.tasks as task}
            <TaskItem {task} showNode />
          {/each}
        {:else}
          <div class="empty-state">No tasks loaded.</div>
        {/if}
      </div>
    </CardContent>
  </Card>
</div>
