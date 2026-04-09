<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import type { Snippet } from 'svelte';
  import { messages } from '$lib/i18n';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNextButton,
    PaginationPrevButton,
  } from '$lib/components/ui/pagination';
  import TaskItem from '$lib/components/app/task-item.svelte';

  interface Props {
    data: PageData;
    children?: Snippet;
  }

  let { data }: Props = $props();

  const pageSize = 20;
  let totalPages = $derived(data.totalCount > 0 ? Math.ceil(data.totalCount / pageSize) : 0);
  let currentPath = $derived($page.url.pathname);
  let currentPage = $state(1);

  $effect(() => {
    currentPage = data.page;
  });

  $effect(() => {
    document.title = `Tasks - Composia`;
  });

  function pageUrl(page: number): string {
    const params = new URLSearchParams();
    params.set('page', page.toString());
    return `${currentPath}?${params.toString()}`;
  }

  $effect(() => {
    if (currentPage === data.page) {
      return;
    }

    void goto(pageUrl(currentPage));
  });
</script>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title">{$messages.tasks.taskHistory}</CardTitle>
        </div>
        <Badge variant="outline">{data.totalCount}</Badge>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
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
          <div class="empty-state">{$messages.tasks.noTasks}</div>
        {/if}
      </div>

      {#if totalPages > 1}
        <div class="mt-6">
          <Pagination count={data.totalCount} perPage={pageSize} bind:page={currentPage}>
            {#snippet children({ pages, currentPage })}
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevButton />
                </PaginationItem>

                {#each pages as page (page.key)}
                  {#if page.type === 'ellipsis'}
                    <PaginationItem>
                      <PaginationEllipsis />
                    </PaginationItem>
                  {:else}
                    <PaginationItem>
                      <PaginationLink {page} isActive={currentPage === page.value} />
                    </PaginationItem>
                  {/if}
                {/each}

                <PaginationItem>
                  <PaginationNextButton />
                </PaginationItem>
              </PaginationContent>
            {/snippet}
          </Pagination>
        </div>
      {/if}
    </CardContent>
  </Card>
</div>
