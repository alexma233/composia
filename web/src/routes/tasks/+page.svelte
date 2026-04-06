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
  import TaskItem from '$lib/components/app/task-item.svelte';

  let { data }: { data: PageData } = $props();

  const pageSize = 20;
  let totalPages = $derived(data.totalCount > 0 ? Math.ceil(data.totalCount / pageSize) : 0);
  let currentPath = $derived($page.url.pathname);

  $effect(() => {
    document.title = `Tasks - Composia`;
  });

  function pageUrl(page: number): string {
    const params = new URLSearchParams();
    params.set('page', page.toString());
    return `${currentPath}?${params.toString()}`;
  }

  let pageNumbers = $derived((() => {
    if (totalPages <= 1) return [];
    if (totalPages <= 7) {
      return Array.from({ length: totalPages }, (_, i) => i + 1);
    }
    const current = data.page;
    const pages: (number | 'ellipsis')[] = [];
    if (current <= 4) {
      for (let i = 1; i <= 5; i++) pages.push(i);
      pages.push('ellipsis');
      pages.push(totalPages);
    } else if (current >= totalPages - 3) {
      pages.push(1);
      pages.push('ellipsis');
      for (let i = totalPages - 4; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1);
      pages.push('ellipsis');
      for (let i = current - 1; i <= current + 1; i++) pages.push(i);
      pages.push('ellipsis');
      pages.push(totalPages);
    }
    return pages;
  })());
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

              {#each pageNumbers as pageNum}
                {#if pageNum === 'ellipsis'}
                  <PaginationItem>
                    <PaginationEllipsis />
                  </PaginationItem>
                {:else}
                  <PaginationItem>
                    <PaginationLink page={pageNum} href={pageUrl(pageNum)} active={pageNum === data.page} />
                  </PaginationItem>
                {/if}
              {/each}

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
