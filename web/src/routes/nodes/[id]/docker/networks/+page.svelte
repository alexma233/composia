<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
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
  import { formatDockerTimestamp, formatShortId } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
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

  let searchQuery = $state('');
  let sortField = $state<'name' | 'driver' | 'created'>('name');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let networks = $state<DockerNetworkSummary[]>([]);
  let removeBusyId = $state('');
  let removeDialogOpen = $state(false);
  let removeTarget = $state<DockerNetworkSummary | null>(null);

  $effect(() => {
    loading = !data.ready;
    loadError = data.error ?? null;
    networks = data.networks || [];
  });

  async function refreshNetworks() {
    if (!data.ready) {
      loading = false;
      return;
    }

    loading = true;
    loadError = null;

    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/networks`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error || 'Failed to load networks');
      }
      networks = payload.networks ?? [];
    } catch (error) {
      loadError = error instanceof Error ? error.message : 'Failed to load networks';
      networks = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void refreshNetworks();
  });

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
      sortField = field as typeof sortField;
      sortDirection = 'asc';
    }
  }

  let filteredNetworks = $derived(networks.filter((n) => {
    const query = searchQuery.toLowerCase();
    return (
      n.name.toLowerCase().includes(query) ||
      n.driver.toLowerCase().includes(query) ||
      n.id.toLowerCase().includes(query) ||
      n.scope.toLowerCase().includes(query)
    );
  }));

  let sortedNetworks = $derived([...filteredNetworks].sort((a, b) => {
    let comparison = 0;
    switch (sortField) {
      case 'name':
        comparison = a.name.localeCompare(b.name);
        break;
      case 'driver':
        comparison = a.driver.localeCompare(b.driver);
        break;
      case 'created':
        comparison = new Date(a.created || 0).getTime() - new Date(b.created || 0).getTime();
        break;
    }
    return sortDirection === 'asc' ? comparison : -comparison;
  }));
</script>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">{$messages.docker.networks.title}</CardTitle>
            <CardDescription class="page-description">
              {$messages.docker.networks.title} on {data.nodeId}
              {#if !loading}
                <Badge variant="secondary" class="ml-2">{networks.length}</Badge>
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            {$messages.common.back}
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <SearchIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder={$messages.docker.networks.searchPlaceholder}
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
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
        {:else if sortedNetworks.length > 0}
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
              {#each sortedNetworks as network}
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
                        <CopyButton text={network.id} label="{$messages.common.copy} ID" />
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
                      <Badge variant="default">{network.containersCount}</Badge>
                    {:else}
                      <span class="text-muted-foreground">0</span>
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
          {#if filteredNetworks.length !== networks.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredNetworks.length} of {networks.length} networks
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
