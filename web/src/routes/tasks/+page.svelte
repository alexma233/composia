<script lang="ts">
  import type { PageData } from './$types';
  import { page } from '$app/stores';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNext,
    PaginationPrev,
  } from '$lib/components/ui/pagination';
  import { taskStatusTone } from '$lib/presenters';
  import TaskItem from '$lib/components/app/task-item.svelte';

  let { data }: { data: PageData } = $props();

  const pageSize = 20;
  let totalPages = $derived(Math.ceil(data.totalCount / pageSize));
  let currentPath = $derived($page.url.pathname);

  $effect(() => {
    document.title = `Tasks - Composia`;
  });

  function pageUrl(page: number): string {
    const params = new URLSearchParams();
    params.set('page', page.toString());
    return `${currentPath}?${params.toString()}`;
  }
</script>

<div class="page-shell">
  <Card class="border-border/70 bg-card/95">
    <CardHeader class="gap-4">
      <div class="flex items-start justify-between gap-4">
        <CardTitle class="page-title">Task history</CardTitle>
        <Badge variant="outline">{data.totalCount}</Badge>
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

      {#if totalPages > 1}
        <div class="mt-6">
          <Pagination>
            <PaginationContent>
              {#if data.page > 1}
                <PaginationItem>
                  <PaginationPrev href={pageUrl(data.page - 1)} />
                </PaginationItem>
              {/if}

              {#each Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                if (totalPages <= 5) return i + 1;
                if (data.page <= 3) return i + 1;
                if (data.page >= totalPages - 2) return totalPages - 4 + i;
                return data.page - 2 + i;
              }) as pageNum}
                <PaginationItem>
                  <PaginationLink page={pageNum} href={pageUrl(pageNum)} active={pageNum === data.page}>
                    {pageNum}
                  </PaginationLink>
                </PaginationItem>
              {/each}

              {#if totalPages > 5 && data.page < totalPages - 2}
                <PaginationEllipsis />
              {/if}

              {#if data.page < totalPages}
                <PaginationItem>
                  <PaginationNext href={pageUrl(data.page + 1)} />
                </PaginationItem>
              {/if}
            </PaginationContent>
          </Pagination>
        </div>
      {/if}
    </CardContent>
  </Card>
</div>
