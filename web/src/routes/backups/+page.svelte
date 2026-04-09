<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { messages } from '$lib/i18n';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNextButton,
    PaginationPrevButton,
  } from '$lib/components/ui/pagination';
  import { formatTimestamp, taskStatusLabel, taskStatusTone } from '$lib/presenters';

  interface Props {
    data: PageData;
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
    document.title = `Backups - Composia`;
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
      <div class="flex items-start justify-between gap-4">
        <CardTitle class="page-title">{$messages.backups.title}</CardTitle>
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
      {#if data.backups.length}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{$messages.backups.backup}</TableHead>
              <TableHead>{$messages.common.status}</TableHead>
              <TableHead>{$messages.nav.tasks}</TableHead>
              <TableHead class="w-56">{$messages.common.finished}</TableHead>
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
                  <Badge variant={taskStatusTone(backup.status)}>{taskStatusLabel(backup.status, $messages)}</Badge>
                </TableCell>
                <TableCell class="text-muted-foreground">{backup.taskId}</TableCell>
                <TableCell class="text-muted-foreground">{formatTimestamp(backup.finishedAt || backup.startedAt)}</TableCell>
              </TableRow>
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">{$messages.backups.noBackups}</div>
      {/if}

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
