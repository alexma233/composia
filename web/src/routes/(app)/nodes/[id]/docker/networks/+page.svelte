<script lang="ts">
  import { goto, invalidate } from '$app/navigation';
  import { page } from '$app/stores';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
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
  import { formatDockerTimestamp, formatShortId } from '$lib/presenters';
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

  type DockerNetworkSummary = {
    id: string;
    name: string;
    driver: string;
    scope: string;
    internal: boolean;
    attachable: boolean;
    created: string;
    labels: Record<string, string>;
    subnet: string;
    gateway: string;
    containersCount: number;
    ipv6Enabled: boolean;
  };

  type DockerNetworkSortField = 'name' | 'driver' | 'created';

  const defaultSortField: DockerNetworkSortField = 'name';

  let searchQuery = $state('');
  let debouncedSearchQuery = $state('');
  let searchDebounceTimer = $state<ReturnType<typeof setTimeout> | null>(null);
  let sortField = $state<DockerNetworkSortField>(defaultSortField);
  let sortDirection = $state<DockerListSortDirection>('asc');
  let currentPage = $state(1);
  let refreshing = $state(false);
  let removeBusyId = $state('');
  let removeDialogOpen = $state(false);
  let removeTarget = $state<DockerNetworkSummary | null>(null);

  let loading = $derived(!data.ready || refreshing);
  let loadError = $derived(data.error ?? null);
  let networks = $derived((data.networks ?? []) as DockerNetworkSummary[]);
  let totalPages = $derived(
    data.totalCount > 0 ? Math.ceil(data.totalCount / data.pageSize) : 0,
  );
  let currentPath = $derived($page.url.pathname);

  $effect(() => {
    refreshing = false;
    currentPage = data.page;
    searchQuery = data.search;
    debouncedSearchQuery = data.search;
    sortField = data.sortBy as DockerNetworkSortField;
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
    nextSortField: DockerNetworkSortField,
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

  async function refreshNetworks() {
    if (!data.ready) {
      return;
    }

    refreshing = true;
    try {
      await invalidate('app:docker-networks');
    } finally {
      refreshing = false;
    }
  }

  function openRemoveDialog(network: DockerNetworkSummary) {
    removeTarget = network;
    removeDialogOpen = true;
  }

  async function queueNetworkRemove() {
    if (!removeTarget) {
      return;
    }

    const network = removeTarget;
    removeBusyId = network.id;
    removeDialogOpen = false;
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/networks/${encodeURIComponent(network.id)}/remove`, {
        method: 'POST'
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.docker.networks.removeFailed));
      }
      toast.success($messages.docker.networks.removeQueued.replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'));
      await refreshNetworks();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.networks.removeFailed);
    } finally {
      removeBusyId = '';
      removeTarget = null;
    }
  }

  let removeDescription = $derived(
    removeTarget ? $messages.docker.networks.removeConfirm.replace('{name}', removeTarget.name) : '',
  );

  function isSystemNetwork(name: string): boolean {
    return name === 'bridge' || name === 'host' || name === 'none';
  }

  function getDriverVariant(driver: string): BadgeVariant {
    const d = driver.toLowerCase();
    if (d === 'bridge' || d === 'host') return 'default';
    if (d === 'overlay') return 'outline';
    if (d === 'macvlan') return 'secondary';
    return 'outline';
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
      sortField = field as DockerNetworkSortField;
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
  <title>{$messages.docker.networks.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <DockerListShell
      title={$messages.docker.networks.title}
      subtitle={$messages.docker.networks.titleOnNode.replace('{nodeId}', data.nodeId)}
      backHref={`/nodes/${data.nodeId}`}
      backLabel={$messages.common.back}
      totalCount={data.totalCount}
      pageSize={data.pageSize}
      itemCount={networks.length}
      {totalPages}
      ready={data.ready}
      {loading}
      error={loadError}
      searchId="network-search"
      searchPlaceholder={$messages.docker.networks.searchPlaceholder}
      loadingText={`${$messages.common.loading} ${$messages.docker.networks.title}...`}
      emptyText={$messages.docker.networks.noNetworks}
      noResultsText={$messages.common.noData}
      countSummary={data.totalCount > networks.length
        ? $messages.docker.networks.countSummary
            .replace('{shown}', String(networks.length))
            .replace('{total}', String(data.totalCount))
        : undefined}
      bind:searchQuery
      bind:currentPage
      onSearchInput={handleSearchInput}
      onClearSearch={clearSearch}
      onRefresh={refreshNetworks}
    >
          <Table>
            <TableCaption class="sr-only">{$messages.docker.networks.tableCaption}</TableCaption>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.common.name} {sortField} {sortDirection} onSort={handleSort} />
                <SortableTableHead field="driver" label={$messages.docker.networks.driver} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.docker.networks.scope}</TableHead>
                <TableHead>{$messages.docker.networks.subnet}</TableHead>
                <TableHead>{$messages.docker.networks.containers}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.common.actions}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each networks as network}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                      <div class="flex items-center gap-2">
                        <a
                          href="/nodes/{data.nodeId}/docker/networks/{encodeURIComponent(network.id)}"
                          class="font-medium hover:underline"
                        >
                          {network.name}
                        </a>
                        {#if isSystemNetwork(network.name)}
                          <Badge variant="secondary" class="text-xs">{$messages.docker.networks.system}</Badge>
                        {/if}
                      </div>
                      <div class="flex items-center gap-1.5">
                        <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded">
                          {formatShortId(network.id)}
                        </code>
                        <CopyButton text={network.id} label={$messages.common.copy} />
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={getDriverVariant(network.driver)}>{network.driver}</Badge>
                  </TableCell>
                  <TableCell>
                    <span class="text-sm">{network.scope}</span>
                  </TableCell>
                  <TableCell>
                    {#if network.subnet}
                      <code class="text-xs bg-muted px-1 py-0.5 rounded">{network.subnet}</code>
                    {:else}
                      <span class="text-muted-foreground">-</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    {#if network.containersCount > 0}
                      <Badge variant="default">{network.containersCount} {$messages.nodes.docker.containers}</Badge>
                    {:else}
                      <Badge variant="secondary">{$messages.docker.networks.unused}</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={network.created}>
                      {formatDockerTimestamp(network.created)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="destructive"
                      size="sm"
                      onclick={() => openRemoveDialog(network)}
                      disabled={removeBusyId === network.id || isSystemNetwork(network.name)}
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
          <DialogTitle>{$messages.docker.networks.removeDialogTitle}</DialogTitle>
          <DialogDescription>{removeDescription}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onclick={() => (removeDialogOpen = false)}>
            {$messages.common.cancel}
          </Button>
          <Button type="button" variant="destructive" onclick={() => void queueNetworkRemove()} disabled={!removeTarget || removeBusyId === removeTarget.id}>
            {$messages.common.delete}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
