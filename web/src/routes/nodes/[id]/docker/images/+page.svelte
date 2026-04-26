<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogOverlay,
    DialogTitle,
  } from '$lib/components/ui/dialog';
  import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNextButton,
    PaginationPrevButton,
  } from '$lib/components/ui/pagination';
  import {
    buildDockerListPageUrl,
    type DockerListSortDirection,
  } from '$lib/docker-list-query';
  import { formatBytes, formatDockerTimestamp, formatShortId } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import { Search } from 'lucide-svelte';
  import { Alert, AlertDescription } from '$lib/components/ui/alert';
  import { messages } from '$lib/i18n';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  type DockerImageSummary = {
    id: string;
    repoTags: string[];
    size: number;
    created: string;
    repoDigests: string[];
    virtualSize: number;
    architecture: string;
    os: string;
    author: string;
    containersCount: number;
    isDangling: boolean;
  };

  type DockerImageSortField = 'name' | 'size' | 'created';

  const defaultSortField: DockerImageSortField = 'name';

  let searchQuery = $state('');
  let sortField = $state<DockerImageSortField>(defaultSortField);
  let sortDirection = $state<DockerListSortDirection>('asc');
  let currentPage = $state(1);
  let refreshing = $state(false);
  let removeBusyId = $state('');
  let removeDialogOpen = $state(false);
  let removeTarget = $state<DockerImageSummary | null>(null);

  let loading = $derived(!data.ready || refreshing);
  let loadError = $derived(data.error ?? null);
  let images = $derived((data.images ?? []) as DockerImageSummary[]);
  let totalPages = $derived(
    data.totalCount > 0 ? Math.ceil(data.totalCount / data.pageSize) : 0,
  );
  let currentPath = $derived($page.url.pathname);

  $effect(() => {
    refreshing = false;
    currentPage = data.page;
    searchQuery = data.search;
    sortField = data.sortBy as DockerImageSortField;
    sortDirection = data.sortDirection as DockerListSortDirection;
  });

  $effect(() => {
    if (!data.ready) {
      return;
    }

    if (
      currentPage === data.page &&
      searchQuery === data.search &&
      sortField === data.sortBy &&
      sortDirection === data.sortDirection
    ) {
      return;
    }

    refreshing = true;
    void goto(pageUrl(currentPage, searchQuery, sortField, sortDirection), {
      keepFocus: true,
      noScroll: true,
      replaceState:
        searchQuery !== data.search ||
        sortField !== data.sortBy ||
        sortDirection !== data.sortDirection,
    });
  });

  function pageUrl(
    pageNumber: number,
    search: string,
    nextSortField: DockerImageSortField,
    nextSortDirection: DockerListSortDirection,
  ) {
    return buildDockerListPageUrl(
      currentPath,
      {
        page: pageNumber,
        search,
        sortBy: nextSortField,
        sortDirection: nextSortDirection,
      },
      defaultSortField,
    );
  }

  async function refreshImages() {
    if (!data.ready) {
      return;
    }

    refreshing = true;
    try {
      await invalidateAll();
    } finally {
      refreshing = false;
    }
  }

  function openRemoveDialog(image: DockerImageSummary) {
    removeTarget = image;
    removeDialogOpen = true;
  }

  async function queueImageRemove() {
    if (!removeTarget) {
      return;
    }

    const image = removeTarget;
    const force = shouldForceRemove(image);
    removeBusyId = image.id;
    removeDialogOpen = false;

    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/images/${encodeURIComponent(image.id)}/remove`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ force })
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? $messages.docker.images.removeFailed);
      }
      toast.success($messages.docker.images.removeQueued.replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'));
      await refreshImages();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.images.removeFailed);
    } finally {
      removeBusyId = '';
      removeTarget = null;
    }
  }

  let removeDescription = $derived(
    removeTarget
      ? shouldForceRemove(removeTarget)
        ? $messages.docker.images.forceRemoveConfirm.replace('{name}', removeTarget.repoTags?.[0] || removeTarget.id)
        : $messages.docker.images.removeConfirm.replace('{name}', removeTarget.repoTags?.[0] || removeTarget.id)
      : '',
  );

  let removeActionLabel = $derived(
    removeTarget && shouldForceRemove(removeTarget)
      ? $messages.docker.images.forceRemoveAction
      : $messages.common.delete,
  );

  function shouldForceRemove(image: DockerImageSummary) {
    return image.containersCount > 0 || (image.repoTags?.length ?? 0) > 1;
  }

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as DockerImageSortField;
      sortDirection = 'asc';
    }
    currentPage = 1;
  }

  function handleSearchInput() {
    currentPage = 1;
  }

  function clearSearch() {
    searchQuery = '';
    currentPage = 1;
  }
</script>

<svelte:head>
  <title>{$messages.docker.images.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="page-header">
          <div class="page-heading">
            <CardTitle class="page-title">{$messages.docker.images.title}</CardTitle>
            <p class="page-description">
              {$messages.docker.images.titleOnNode.replace('{nodeId}', data.nodeId)}
              {#if !loading}
                <Badge variant="outline" class="ml-2">{data.totalCount}</Badge>
              {/if}
            </p>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground transition-colors hover:text-foreground">
            {$messages.common.back}
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder={$messages.docker.images.searchPlaceholder}
              class="pl-9"
              bind:value={searchQuery}
              oninput={handleSearchInput}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={clearSearch}>
              {$messages.common.cancel}
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void refreshImages()} disabled={loading || !data.ready}>
            {#if loading}{$messages.common.loading}...{:else}{$messages.common.refresh}{/if}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {#if loadError}
          <Alert variant="destructive">
            <AlertDescription>{loadError}</AlertDescription>
          </Alert>
        {:else if loading}
          <div class="flex min-h-[320px] items-center justify-center">
            <div class="flex items-center gap-3 text-sm text-muted-foreground">
              <Spinner />
              <span>{$messages.common.loading} {$messages.docker.images.title}...</span>
            </div>
          </div>
        {:else if images.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.docker.images.repository} {sortField} {sortDirection} onSort={handleSort} class="w-[40%]" />
                <SortableTableHead field="size" label={$messages.docker.images.size} {sortField} {sortDirection} onSort={handleSort} class="w-[15%]" />
                <TableHead class="w-[20%]">{$messages.docker.images.architecture}</TableHead>
                <TableHead class="w-[15%]">{$messages.docker.images.usage}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} class="w-[15%]" />
                <TableHead class="w-[10%]">{$messages.common.actions}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each images as image}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                      {#if image.repoTags && image.repoTags.length > 0}
                          <a
                            href="/nodes/{data.nodeId}/docker/images/{encodeURIComponent(image.id)}"
                            class="font-medium truncate max-w-[250px] block hover:underline"
                            title={image.repoTags[0]}
                          >
                            {image.repoTags[0]}
                          </a>
                        {#if image.repoTags.length > 1}
                          <div class="text-xs text-muted-foreground">+{image.repoTags.length - 1} {$messages.docker.images.moreTags}</div>
                        {/if}
                      {:else if image.isDangling}
                        <a
                          href="/nodes/{data.nodeId}/docker/images/{encodeURIComponent(image.id)}"
                          class="font-medium text-muted-foreground hover:underline"
                        >&lt;{$messages.common.none}&gt;</a>
                        <Badge variant="secondary" class="text-xs">{$messages.docker.images.dangling}</Badge>
                      {:else}
                        <a
                          href="/nodes/{data.nodeId}/docker/images/{encodeURIComponent(image.id)}"
                          class="font-medium hover:underline"
                        >{image.id}</a>
                      {/if}
                      <div class="flex items-center gap-1.5">
                        <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded">
                          {formatShortId(image.id)}
                        </code>
                        <CopyButton text={image.id} label={$messages.common.copy + ' ID'} />
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                      <span class="text-sm">{formatBytes(image.size)}</span>
                  </TableCell>
                  <TableCell>
                    <div class="text-sm">
                      {#if image.architecture}
                        <Badge variant="outline">{image.architecture}</Badge>
                      {:else}
                        <span class="text-muted-foreground">-</span>
                      {/if}
                    </div>
                  </TableCell>
                  <TableCell>
                    {#if image.containersCount && image.containersCount > 0}
                      <Badge variant="default">{image.containersCount} {$messages.nodes.docker.containers}</Badge>
                    {:else}
                      <Badge variant="secondary">{$messages.docker.images.unused}</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                      <div class="text-sm text-muted-foreground" title={image.created}>
                        {formatDockerTimestamp(image.created)}
                      </div>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="destructive"
                      size="sm"
                      onclick={() => openRemoveDialog(image)}
                      disabled={removeBusyId === image.id}
                    >
                      {$messages.common.delete}
                    </Button>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if data.totalCount > images.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              {$messages.docker.images.countSummary.replace('{shown}', String(images.length)).replace('{total}', String(data.totalCount))}
            </div>
          {/if}

          {#if totalPages > 1}
            <div class="mt-6">
              <Pagination count={data.totalCount} perPage={data.pageSize} bind:page={currentPage}>
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
        {:else if searchQuery}
          <div class="empty-state">
            {$messages.common.noData}
          </div>
        {:else}
          <div class="empty-state">{$messages.docker.images.noImages}</div>
        {/if}
      </CardContent>
    </Card>

    <Dialog bind:open={removeDialogOpen}>
      <DialogOverlay />
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{$messages.docker.images.removeDialogTitle}</DialogTitle>
          <DialogDescription>{removeDescription}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onclick={() => (removeDialogOpen = false)}>
            {$messages.common.cancel}
          </Button>
          <Button type="button" variant="destructive" onclick={() => void queueImageRemove()} disabled={!removeTarget || removeBusyId === removeTarget.id}>
            {removeActionLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
