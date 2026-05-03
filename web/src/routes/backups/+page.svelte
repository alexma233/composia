<script lang="ts">
  import type { PageData } from './$types';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import { buttonVariants } from '$lib/components/ui/button';
  import { onMount, tick } from 'svelte';
  import { Check, ChevronsUpDown, Filter } from 'lucide-svelte';
  import { messages } from '$lib/i18n';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import * as Command from '$lib/components/ui/command';
  import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNextButton,
    PaginationPrevButton,
  } from '$lib/components/ui/pagination';
  import * as Popover from '$lib/components/ui/popover';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { startPolling } from '$lib/refresh';
  import { formatTimestamp, taskStatusLabel, taskStatusTone } from '$lib/presenters';
  import { cn } from '$lib/utils';

  interface Props {
    data: PageData;
  }

  type BackupStatusFilter = 'pending' | 'running' | 'awaiting_confirmation' | 'succeeded' | 'failed' | 'cancelled';

  type FilterOption = {
    value: string;
    label: string;
    secondary?: string;
  };

  type FilterMode = 'include' | 'exclude';
  type FilterKind = 'status' | 'service' | 'node' | 'data';

  let { data }: Props = $props();

  const pageSize = 20;
  const statusOptions: BackupStatusFilter[] = ['pending', 'running', 'awaiting_confirmation', 'succeeded', 'failed', 'cancelled'];

  let totalPages = $derived(data.totalCount > 0 ? Math.ceil(data.totalCount / pageSize) : 0);
  let currentPath = $derived($page.url.pathname);
  let currentPage = $state(1);
  let currentStatuses = $state<BackupStatusFilter[]>([]);
  let excludedStatuses = $state<BackupStatusFilter[]>([]);
  let currentServiceNames = $state<string[]>([]);
  let excludedServiceNames = $state<string[]>([]);
  let currentNodeIds = $state<string[]>([]);
  let excludedNodeIds = $state<string[]>([]);
  let currentDataNames = $state<string[]>([]);
  let excludedDataNames = $state<string[]>([]);

  let filterOpen = $state(false);
  let statusSelectOpen = $state(false);
  let serviceSelectOpen = $state(false);
  let nodeSelectOpen = $state(false);
  let dataSelectOpen = $state(false);
  let statusMode = $state<FilterMode>('include');
  let serviceMode = $state<FilterMode>('include');
  let nodeMode = $state<FilterMode>('include');
  let dataMode = $state<FilterMode>('include');

  let statusTriggerRef = $state<HTMLButtonElement | null>(null);
  let serviceTriggerRef = $state<HTMLButtonElement | null>(null);
  let nodeTriggerRef = $state<HTMLButtonElement | null>(null);
  let dataTriggerRef = $state<HTMLButtonElement | null>(null);

  const activeFilterCount = $derived(
    [
      currentStatuses.length > 0 || excludedStatuses.length > 0,
      currentServiceNames.length > 0 || excludedServiceNames.length > 0,
      currentNodeIds.length > 0 || excludedNodeIds.length > 0,
      currentDataNames.length > 0 || excludedDataNames.length > 0,
    ].filter(Boolean).length,
  );

  const serviceOptions = $derived(
    data.services.map((service) => ({
      value: service.name,
      label: service.name,
    })) satisfies FilterOption[],
  );

  const nodeOptions = $derived(
    data.nodes.map((node) => ({
      value: node.nodeId,
      label: node.displayName,
      secondary: node.nodeId,
    })) satisfies FilterOption[],
  );

  const dataOptions = $derived(dataNameOptions(data.backups.map((backup) => backup.dataName), currentDataNames, excludedDataNames));

  const statusSummary = $derived(selectionSummary(currentStatuses, excludedStatuses, statusOptions.map((status) => ({ value: status, label: statusLabel(status) })), $messages.tasks.filters.allStatuses));
  const serviceSummary = $derived(selectionSummary(currentServiceNames, excludedServiceNames, serviceOptions, $messages.tasks.filters.allServices));
  const nodeSummary = $derived(selectionSummary(currentNodeIds, excludedNodeIds, nodeOptions, $messages.tasks.filters.allNodes));
  const dataSummary = $derived(selectionSummary(currentDataNames, excludedDataNames, dataOptions, $messages.backups.filters.allDataNames));

  const emptyMessage = $derived(activeFilterCount === 0 ? $messages.backups.noBackups : $messages.backups.noBackupsForFilter);

  $effect(() => {
    currentPage = data.page;
    currentStatuses = [...data.status] as BackupStatusFilter[];
    excludedStatuses = [...data.excludeStatus] as BackupStatusFilter[];
    currentServiceNames = [...data.serviceName];
    excludedServiceNames = [...data.excludeServiceName];
    currentNodeIds = [...data.nodeId];
    excludedNodeIds = [...data.excludeNodeId];
    currentDataNames = [...data.dataName];
    excludedDataNames = [...data.excludeDataName];
  });

  onMount(() => startPolling(() => invalidateAll(), { intervalMs: 5000 }));

  function statusLabel(status: BackupStatusFilter): string {
    return taskStatusLabel(status, $messages);
  }

  function dataNameOptions(loadedValues: string[], includeValues: string[], excludeValues: string[]): FilterOption[] {
    const values = Array.from(new Set([...includeValues, ...excludeValues, ...loadedValues].filter(Boolean))).sort((left, right) => left.localeCompare(right));
    return values.map((value) => ({ value, label: value }));
  }

  function selectionSummary(includeValues: string[], excludeValues: string[], options: FilterOption[], emptyLabel: string): string {
    if (includeValues.length === 0 && excludeValues.length === 0) {
      return emptyLabel;
    }

    if (includeValues.length > 0 && excludeValues.length === 0) {
      if (includeValues.length === 1) {
        return `${$messages.tasks.filters.include} ${options.find((option) => option.value === includeValues[0])?.label ?? includeValues[0]}`;
      }
      return `${includeValues.length} ${$messages.tasks.filters.includeSelectedCount}`;
    }

    if (excludeValues.length > 0 && includeValues.length === 0) {
      if (excludeValues.length === 1) {
        return `${$messages.tasks.filters.exclude} ${options.find((option) => option.value === excludeValues[0])?.label ?? excludeValues[0]}`;
      }
      return `${excludeValues.length} ${$messages.tasks.filters.excludeSelectedCount}`;
    }

    return `${includeValues.length} ${$messages.tasks.filters.includeSelectedCount} / ${excludeValues.length} ${$messages.tasks.filters.excludeSelectedCount}`;
  }

  function toggleValue<T extends string>(values: T[], value: T): T[] {
    return values.includes(value) ? values.filter((entry) => entry !== value) : [...values, value];
  }

  function clearModeValues(kind: FilterKind, mode: FilterMode) {
    if (kind === 'status') {
      if (mode === 'include') currentStatuses = [];
      if (mode === 'exclude') excludedStatuses = [];
    }
    if (kind === 'service') {
      if (mode === 'include') currentServiceNames = [];
      if (mode === 'exclude') excludedServiceNames = [];
    }
    if (kind === 'node') {
      if (mode === 'include') currentNodeIds = [];
      if (mode === 'exclude') excludedNodeIds = [];
    }
    if (kind === 'data') {
      if (mode === 'include') currentDataNames = [];
      if (mode === 'exclude') excludedDataNames = [];
    }
  }

  function toggleFilterSelection<T extends string>(kind: FilterKind, mode: FilterMode, value: T) {
    if (kind === 'status') {
      if (mode === 'include') {
        currentStatuses = toggleValue(currentStatuses as T[], value) as BackupStatusFilter[];
        excludedStatuses = excludedStatuses.filter((entry) => entry !== value) as BackupStatusFilter[];
      } else {
        excludedStatuses = toggleValue(excludedStatuses as T[], value) as BackupStatusFilter[];
        currentStatuses = currentStatuses.filter((entry) => entry !== value) as BackupStatusFilter[];
      }
    }
    if (kind === 'service') {
      if (mode === 'include') {
        currentServiceNames = toggleValue(currentServiceNames as T[], value) as string[];
        excludedServiceNames = excludedServiceNames.filter((entry) => entry !== value) as string[];
      } else {
        excludedServiceNames = toggleValue(excludedServiceNames as T[], value) as string[];
        currentServiceNames = currentServiceNames.filter((entry) => entry !== value) as string[];
      }
    }
    if (kind === 'node') {
      if (mode === 'include') {
        currentNodeIds = toggleValue(currentNodeIds as T[], value) as string[];
        excludedNodeIds = excludedNodeIds.filter((entry) => entry !== value) as string[];
      } else {
        excludedNodeIds = toggleValue(excludedNodeIds as T[], value) as string[];
        currentNodeIds = currentNodeIds.filter((entry) => entry !== value) as string[];
      }
    }
    if (kind === 'data') {
      if (mode === 'include') {
        currentDataNames = toggleValue(currentDataNames as T[], value) as string[];
        excludedDataNames = excludedDataNames.filter((entry) => entry !== value) as string[];
      } else {
        excludedDataNames = toggleValue(excludedDataNames as T[], value) as string[];
        currentDataNames = currentDataNames.filter((entry) => entry !== value) as string[];
      }
    }
  }

  function isSelected(kind: FilterKind, mode: FilterMode, value: string): boolean {
    if (kind === 'status') return mode === 'include' ? currentStatuses.includes(value as BackupStatusFilter) : excludedStatuses.includes(value as BackupStatusFilter);
    if (kind === 'service') return mode === 'include' ? currentServiceNames.includes(value) : excludedServiceNames.includes(value);
    if (kind === 'node') return mode === 'include' ? currentNodeIds.includes(value) : excludedNodeIds.includes(value);
    return mode === 'include' ? currentDataNames.includes(value) : excludedDataNames.includes(value);
  }

  function hasModeValues(kind: FilterKind, mode: FilterMode): boolean {
    if (kind === 'status') return mode === 'include' ? currentStatuses.length > 0 : excludedStatuses.length > 0;
    if (kind === 'service') return mode === 'include' ? currentServiceNames.length > 0 : excludedServiceNames.length > 0;
    if (kind === 'node') return mode === 'include' ? currentNodeIds.length > 0 : excludedNodeIds.length > 0;
    return mode === 'include' ? currentDataNames.length > 0 : excludedDataNames.length > 0;
  }

  function closeAndFocus(kind: FilterKind) {
    if (kind === 'status') statusSelectOpen = false;
    if (kind === 'service') serviceSelectOpen = false;
    if (kind === 'node') nodeSelectOpen = false;
    if (kind === 'data') dataSelectOpen = false;

    tick().then(() => {
      if (kind === 'status') statusTriggerRef?.focus();
      if (kind === 'service') serviceTriggerRef?.focus();
      if (kind === 'node') nodeTriggerRef?.focus();
      if (kind === 'data') dataTriggerRef?.focus();
    });
  }

  function modeLabel(mode: FilterMode): string {
    return mode === 'include' ? $messages.tasks.filters.include : $messages.tasks.filters.exclude;
  }

  function pageUrl(page: number, statuses: string[], excludeStatuses: string[], serviceNames: string[], excludeServiceNames: string[], nodeIds: string[], excludeNodeIds: string[], dataNames: string[], excludeDataNames: string[]): string {
    const params = new URLSearchParams();
    if (page > 1) {
      params.set('page', page.toString());
    }
    for (const status of statuses) params.append('status', status);
    for (const status of excludeStatuses) params.append('excludeStatus', status);
    for (const serviceName of serviceNames) params.append('serviceName', serviceName);
    for (const serviceName of excludeServiceNames) params.append('excludeServiceName', serviceName);
    for (const nodeId of nodeIds) params.append('nodeId', nodeId);
    for (const nodeId of excludeNodeIds) params.append('excludeNodeId', nodeId);
    for (const dataName of dataNames) params.append('dataName', dataName);
    for (const dataName of excludeDataNames) params.append('excludeDataName', dataName);

    const query = params.toString();
    return query ? `${currentPath}?${query}` : currentPath;
  }

  function currentFilterUrl(page: number): string {
    return pageUrl(page, currentStatuses, excludedStatuses, currentServiceNames, excludedServiceNames, currentNodeIds, excludedNodeIds, currentDataNames, excludedDataNames);
  }

  function applyFilters() {
    currentPage = 1;
    filterOpen = false;
    void goto(currentFilterUrl(1));
  }

  function resetFilters() {
    currentStatuses = [];
    excludedStatuses = [];
    currentServiceNames = [];
    excludedServiceNames = [];
    currentNodeIds = [];
    excludedNodeIds = [];
    currentDataNames = [];
    excludedDataNames = [];
    currentPage = 1;
    filterOpen = false;
    void goto(currentPath);
  }

  $effect(() => {
    if (
      currentPage === data.page
      && JSON.stringify(currentStatuses) === JSON.stringify(data.status)
      && JSON.stringify(excludedStatuses) === JSON.stringify(data.excludeStatus)
      && JSON.stringify(currentServiceNames) === JSON.stringify(data.serviceName)
      && JSON.stringify(excludedServiceNames) === JSON.stringify(data.excludeServiceName)
      && JSON.stringify(currentNodeIds) === JSON.stringify(data.nodeId)
      && JSON.stringify(excludedNodeIds) === JSON.stringify(data.excludeNodeId)
      && JSON.stringify(currentDataNames) === JSON.stringify(data.dataName)
      && JSON.stringify(excludedDataNames) === JSON.stringify(data.excludeDataName)
    ) {
      return;
    }

    if (currentPage !== data.page) {
      void goto(currentFilterUrl(currentPage));
    }
  });
</script>

<svelte:head>
  <title>{$messages.backups.title} - {$messages.app.name}</title>
  <meta
    name="description"
    content={$messages.backups.pageDescription}
  />
</svelte:head>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title">{$messages.backups.title}</CardTitle>
        </div>
        <div class="flex items-center gap-2">
          <Popover.Root bind:open={filterOpen}>
            <Popover.Trigger class="inline-flex">
              {#snippet child({ props })}
                <Button type="button" variant="outline" class="gap-2" {...props}>
                  <Filter class="size-4" />
                  {$messages.common.filter}
                  {#if activeFilterCount > 0}
                    <Badge variant="secondary">{activeFilterCount}</Badge>
                  {/if}
                </Button>
              {/snippet}
            </Popover.Trigger>

            <Popover.Content class="w-[min(92vw,52rem)] p-4" align="end" sideOffset={8}>
              <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                <div class="space-y-2">
                  <div class="text-sm text-muted-foreground">{$messages.common.status}</div>
                  <Popover.Root bind:open={statusSelectOpen}>
                    <Popover.Trigger bind:ref={statusTriggerRef} class={buttonVariants({ variant: 'outline', class: 'w-full justify-between font-normal' })}>
                      <span class="truncate">{statusSummary}</span>
                      <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
                    </Popover.Trigger>
                    <Popover.Content class="w-[min(92vw,20rem)] p-0" align="start">
                      <div class="flex border-b p-1 gap-1">
                        <Button type="button" size="sm" variant={statusMode === 'include' ? 'default' : 'ghost'} onclick={() => { statusMode = 'include'; }}>{modeLabel('include')}</Button>
                        <Button type="button" size="sm" variant={statusMode === 'exclude' ? 'default' : 'ghost'} onclick={() => { statusMode = 'exclude'; }}>{modeLabel('exclude')}</Button>
                      </div>
                      <Command.Root>
                        <Command.Input placeholder={$messages.tasks.filters.searchStatusPlaceholder} />
                        <Command.List>
                          <Command.Empty>{$messages.tasks.filters.noStatusesFound}</Command.Empty>
                          <Command.Group>
                            <Command.Item value="__all__" onSelect={() => { clearModeValues('status', statusMode); closeAndFocus('status'); }}>
                              <span>{statusMode === 'include' ? $messages.tasks.filters.clearIncluded : $messages.tasks.filters.clearExcluded}</span>
                              <Check class={cn('ml-auto size-4', hasModeValues('status', statusMode) && 'text-transparent')} />
                            </Command.Item>
                            {#each statusOptions as status}
                              <Command.Item value={status} onSelect={() => { toggleFilterSelection('status', statusMode, status); }}>
                                <span>{statusLabel(status)}</span>
                                <Check class={cn('ml-auto size-4', !isSelected('status', statusMode, status) && 'text-transparent')} />
                              </Command.Item>
                            {/each}
                          </Command.Group>
                        </Command.List>
                      </Command.Root>
                    </Popover.Content>
                  </Popover.Root>
                </div>

                <div class="space-y-2">
                  <div class="text-sm text-muted-foreground">{$messages.tasks.filters.serviceName}</div>
                  <Popover.Root bind:open={serviceSelectOpen}>
                    <Popover.Trigger bind:ref={serviceTriggerRef} class={buttonVariants({ variant: 'outline', class: 'w-full justify-between font-normal' })}>
                      <span class="truncate">{serviceSummary}</span>
                      <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
                    </Popover.Trigger>
                    <Popover.Content class="w-[min(92vw,22rem)] p-0" align="start">
                      <div class="flex border-b p-1 gap-1">
                        <Button type="button" size="sm" variant={serviceMode === 'include' ? 'default' : 'ghost'} onclick={() => { serviceMode = 'include'; }}>{modeLabel('include')}</Button>
                        <Button type="button" size="sm" variant={serviceMode === 'exclude' ? 'default' : 'ghost'} onclick={() => { serviceMode = 'exclude'; }}>{modeLabel('exclude')}</Button>
                      </div>
                      <Command.Root>
                        <Command.Input placeholder={$messages.tasks.filters.searchServicePlaceholder} />
                        <Command.List>
                          <Command.Empty>{$messages.tasks.filters.noServicesFound}</Command.Empty>
                          <Command.Group>
                            <Command.Item value="__all__" onSelect={() => { clearModeValues('service', serviceMode); closeAndFocus('service'); }}>
                              <span>{serviceMode === 'include' ? $messages.tasks.filters.clearIncluded : $messages.tasks.filters.clearExcluded}</span>
                              <Check class={cn('ml-auto size-4', hasModeValues('service', serviceMode) && 'text-transparent')} />
                            </Command.Item>
                            {#each serviceOptions as service (service.value)}
                              <Command.Item value={service.value} onSelect={() => { toggleFilterSelection('service', serviceMode, service.value); }}>
                                <span class="truncate">{service.label}</span>
                                <Check class={cn('ml-auto size-4', !isSelected('service', serviceMode, service.value) && 'text-transparent')} />
                              </Command.Item>
                            {/each}
                          </Command.Group>
                        </Command.List>
                      </Command.Root>
                    </Popover.Content>
                  </Popover.Root>
                </div>

                <div class="space-y-2">
                  <div class="text-sm text-muted-foreground">{$messages.tasks.filters.nodeId}</div>
                  <Popover.Root bind:open={nodeSelectOpen}>
                    <Popover.Trigger bind:ref={nodeTriggerRef} class={buttonVariants({ variant: 'outline', class: 'w-full justify-between font-normal' })}>
                      <span class="truncate">{nodeSummary}</span>
                      <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
                    </Popover.Trigger>
                    <Popover.Content class="w-[min(92vw,22rem)] p-0" align="start">
                      <div class="flex border-b p-1 gap-1">
                        <Button type="button" size="sm" variant={nodeMode === 'include' ? 'default' : 'ghost'} onclick={() => { nodeMode = 'include'; }}>{modeLabel('include')}</Button>
                        <Button type="button" size="sm" variant={nodeMode === 'exclude' ? 'default' : 'ghost'} onclick={() => { nodeMode = 'exclude'; }}>{modeLabel('exclude')}</Button>
                      </div>
                      <Command.Root>
                        <Command.Input placeholder={$messages.tasks.filters.searchNodePlaceholder} />
                        <Command.List>
                          <Command.Empty>{$messages.tasks.filters.noNodesFound}</Command.Empty>
                          <Command.Group>
                            <Command.Item value="__all__" onSelect={() => { clearModeValues('node', nodeMode); closeAndFocus('node'); }}>
                              <span>{nodeMode === 'include' ? $messages.tasks.filters.clearIncluded : $messages.tasks.filters.clearExcluded}</span>
                              <Check class={cn('ml-auto size-4', hasModeValues('node', nodeMode) && 'text-transparent')} />
                            </Command.Item>
                            {#each nodeOptions as node (node.value)}
                              <Command.Item value={`${node.label} ${node.secondary ?? ''}`} onSelect={() => { toggleFilterSelection('node', nodeMode, node.value); }}>
                                <div class="min-w-0">
                                  <div class="truncate">{node.label}</div>
                                  {#if node.secondary}
                                    <div class="truncate text-xs text-muted-foreground">{node.secondary}</div>
                                  {/if}
                                </div>
                                <Check class={cn('ml-auto size-4', !isSelected('node', nodeMode, node.value) && 'text-transparent')} />
                              </Command.Item>
                            {/each}
                          </Command.Group>
                        </Command.List>
                      </Command.Root>
                    </Popover.Content>
                  </Popover.Root>
                </div>

                <div class="space-y-2">
                  <div class="text-sm text-muted-foreground">{$messages.backups.dataName}</div>
                  <Popover.Root bind:open={dataSelectOpen}>
                    <Popover.Trigger bind:ref={dataTriggerRef} class={buttonVariants({ variant: 'outline', class: 'w-full justify-between font-normal' })}>
                      <span class="truncate">{dataSummary}</span>
                      <ChevronsUpDown class="size-4 shrink-0 opacity-50" />
                    </Popover.Trigger>
                    <Popover.Content class="w-[min(92vw,22rem)] p-0" align="start">
                      <div class="flex border-b p-1 gap-1">
                        <Button type="button" size="sm" variant={dataMode === 'include' ? 'default' : 'ghost'} onclick={() => { dataMode = 'include'; }}>{modeLabel('include')}</Button>
                        <Button type="button" size="sm" variant={dataMode === 'exclude' ? 'default' : 'ghost'} onclick={() => { dataMode = 'exclude'; }}>{modeLabel('exclude')}</Button>
                      </div>
                      <Command.Root>
                        <Command.Input placeholder={$messages.backups.filters.searchDataPlaceholder} />
                        <Command.List>
                          <Command.Empty>{$messages.backups.filters.noDataFound}</Command.Empty>
                          <Command.Group>
                            <Command.Item value="__all__" onSelect={() => { clearModeValues('data', dataMode); closeAndFocus('data'); }}>
                              <span>{dataMode === 'include' ? $messages.tasks.filters.clearIncluded : $messages.tasks.filters.clearExcluded}</span>
                              <Check class={cn('ml-auto size-4', hasModeValues('data', dataMode) && 'text-transparent')} />
                            </Command.Item>
                            {#each dataOptions as dataName (dataName.value)}
                              <Command.Item value={dataName.value} onSelect={() => { toggleFilterSelection('data', dataMode, dataName.value); }}>
                                <span class="truncate">{dataName.label}</span>
                                <Check class={cn('ml-auto size-4', !isSelected('data', dataMode, dataName.value) && 'text-transparent')} />
                              </Command.Item>
                            {/each}
                          </Command.Group>
                        </Command.List>
                      </Command.Root>
                    </Popover.Content>
                  </Popover.Root>
                </div>
              </div>

              <div class="mt-4 flex items-center justify-end gap-2">
                <Button type="button" variant="outline" onclick={resetFilters}>{$messages.common.reset}</Button>
                <Button type="button" onclick={applyFilters}>{$messages.common.apply}</Button>
              </div>
            </Popover.Content>
          </Popover.Root>

          <Badge variant="outline">{data.totalCount}</Badge>
        </div>
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
              <TableRow class="hover:bg-accent/50">
                <TableCell>
                  <a href={`/backups/${backup.backupId}`} class="font-medium hover:text-primary">{backup.serviceName} / {backup.dataName}</a>
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
        <div class="empty-state">{emptyMessage}</div>
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
