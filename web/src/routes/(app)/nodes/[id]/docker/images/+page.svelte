<script lang="ts">
  import { goto, invalidate } from '$app/navigation';
  import { page } from '$app/stores';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
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
    buildDockerListPageUrl,
    debouncedDockerListSearchState,
    dockerSearchDebounceMs,
    type DockerListSortDirection,
  } from '$lib/docker-list-query';
  import { formatBytes, formatDockerTimestamp, formatShortId } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import DockerListShell from '$lib/components/app/docker-list-shell.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import { actionErrorMessage } from '$lib/capabilities';
  import { getMessages } from '$lib/i18n';

  const messages = getMessages();

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
  let debouncedSearchQuery = $state('');
  let searchDebounceTimer = $state<ReturnType<typeof setTimeout> | null>(null);
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
    debouncedSearchQuery = data.search;
    sortField = data.sortBy as DockerImageSortField;
    sortDirection = data.sortDirection as DockerListSortDirection;
  });

  $effect(() => {
    if (!data.ready) {
      return;
    }

    if (
      currentPage === data.page &&
      debouncedSearchQuery === data.search &&
      sortField === data.sortBy &&
      sortDirection === data.sortDirection
    ) {
      return;
    }

    refreshing = true;
    void goto(pageUrl(currentPage, debouncedSearchQuery, sortField, sortDirection), {
      keepFocus: true,
      noScroll: true,
      replaceState:
        debouncedSearchQuery !== data.search ||
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
      await invalidate('app:docker-images');
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
        throw new Error(actionErrorMessage(payload, $messages, $messages.docker.images.removeFailed));
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

  function flushSearchDebounce() {
    if (searchDebounceTimer) {
      clearTimeout(searchDebounceTimer);
      searchDebounceTimer = null;
    }
    debouncedSearchQuery = searchQuery;
  }

  function handleSort(field: string) {
    flushSearchDebounce();
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as DockerImageSortField;
      sortDirection = 'asc';
    }
    currentPage = 1;
  }

  function handleSearchInput() {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
      const nextSearch = debouncedDockerListSearchState(searchQuery, debouncedSearchQuery);
      if (nextSearch) {
        currentPage = nextSearch.page;
        debouncedSearchQuery = nextSearch.search;
      }
      searchDebounceTimer = null;
    }, dockerSearchDebounceMs);
  }

  function clearSearch() {
    searchQuery = '';
    currentPage = 1;
    flushSearchDebounce();
  }
</script>

<svelte:head>
  <title>{$messages.docker.images.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <DockerListShell
      title={$messages.docker.images.title}
      subtitle={$messages.docker.images.titleOnNode.replace('{nodeId}', data.nodeId)}
      backHref={`/nodes/${data.nodeId}`}
      backLabel={$messages.common.back}
      totalCount={data.totalCount}
      pageSize={data.pageSize}
      itemCount={images.length}
      {totalPages}
      ready={data.ready}
      {loading}
      error={loadError}
      searchId="image-search"
      searchPlaceholder={$messages.docker.images.searchPlaceholder}
      loadingText={`${$messages.common.loading} ${$messages.docker.images.title}...`}
      emptyText={$messages.docker.images.noImages}
      noResultsText={$messages.common.noData}
      countSummary={data.totalCount > images.length
        ? $messages.docker.images.countSummary
            .replace('{shown}', String(images.length))
            .replace('{total}', String(data.totalCount))
        : undefined}
      bind:searchQuery
      bind:currentPage
      onSearchInput={handleSearchInput}
      onClearSearch={clearSearch}
      onRefresh={refreshImages}
    >
          <Table>
            <TableCaption class="sr-only">{$messages.docker.images.tableCaption}</TableCaption>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.docker.images.repository} {sortField} {sortDirection} onSort={handleSort} />
                <SortableTableHead field="size" label={$messages.docker.images.size} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.docker.images.architecture}</TableHead>
                <TableHead>{$messages.docker.images.usage}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.common.actions}</TableHead>
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
                        <CopyButton text={image.id} label={$messages.common.copy} />
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
    </DockerListShell>

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
