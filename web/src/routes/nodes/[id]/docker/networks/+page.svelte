<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
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
  import { formatDockerTimestamp, formatShortId } from '$lib/presenters';
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
    sortField = data.sortBy as DockerNetworkSortField;
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
      await invalidateAll();
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
        throw new Error(payload.error ?? $messages.docker.networks.removeFailed);
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

  let removeActionLabel = $derived($messages.common.delete);

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

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as DockerNetworkSortField;
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
  <title>{$messages.docker.networks.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="page-header">
          <div class="page-heading">
            <CardTitle class="page-title" level="1">{$messages.docker.networks.title}</CardTitle>
            <p class="page-description">
              {$messages.docker.networks.titleOnNode.replace('{nodeId}', data.nodeId)}
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
              placeholder={$messages.docker.networks.searchPlaceholder}
              aria-label={$messages.docker.networks.searchPlaceholder}
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
          <Button variant="outline" size="sm" onclick={() => void refreshNetworks()} disabled={loading || !data.ready}>
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
              <span>{$messages.common.loading} {$messages.docker.networks.title}...</span>
            </div>
          </div>
        {:else if networks.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.common.name} {sortField} {sortDirection} onSort={handleSort} class="w-[30%]" />
                <SortableTableHead field="driver" label={$messages.docker.networks.driver} {sortField} {sortDirection} onSort={handleSort} class="w-[10%]" />
                <TableHead class="w-[10%]">{$messages.docker.networks.scope}</TableHead>
                <TableHead class="w-[15%]">{$messages.docker.networks.subnet}</TableHead>
                <TableHead class="w-[10%]">{$messages.docker.networks.containers}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} class="w-[20%]" />
                <TableHead class="w-[10%]">{$messages.common.actions}</TableHead>
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
            {removeActionLabel}
                    </Button>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if data.totalCount > networks.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              {$messages.docker.networks.countSummary.replace('{shown}', String(networks.length)).replace('{total}', String(data.totalCount))}
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
          <div class="empty-state">{$messages.docker.networks.noNetworks}</div>
        {/if}
      </CardContent>
    </Card>

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
