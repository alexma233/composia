<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { buttonVariants } from '$lib/components/ui/button';
  import type { Snippet } from 'svelte';
  import { tick } from 'svelte';
  import CheckIcon from '@lucide/svelte/icons/check';
  import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
  import FilterIcon from '@lucide/svelte/icons/filter';
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
  import { cn } from '$lib/utils';
  import TaskItem from '$lib/components/app/task-item.svelte';

  interface Props {
    data: PageData;
    children?: Snippet;
  }

  type TaskStatusFilter = 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled';
  type TaskTypeFilter =
    | 'deploy'
    | 'update'
    | 'restart'
    | 'stop'
    | 'backup'
    | 'restore'
    | 'migrate'
    | 'dns_update'
    | 'caddy_sync'
    | 'caddy_reload'
    | 'prune'
    | 'rustic_forget'
    | 'rustic_prune'
    | 'docker_list'
    | 'docker_inspect'
    | 'docker_start'
    | 'docker_stop'
    | 'docker_restart'
    | 'docker_logs';

  type FilterOption = {
    value: string;
    label: string;
    secondary?: string;
  };

  type FilterMode = 'include' | 'exclude';

  const defaultExcludedTypes: TaskTypeFilter[] = ['docker_list', 'docker_inspect'];

  let { data }: Props = $props();

  const pageSize = 20;
  const statusOptions: TaskStatusFilter[] = ['pending', 'running', 'succeeded', 'failed', 'cancelled'];
  const taskTypeOptions: TaskTypeFilter[] = [
    'deploy',
    'update',
    'restart',
    'stop',
    'backup',
    'restore',
    'migrate',
    'dns_update',
    'caddy_sync',
    'caddy_reload',
    'prune',
    'rustic_forget',
    'rustic_prune',
    'docker_list',
    'docker_inspect',
    'docker_start',
    'docker_stop',
    'docker_restart',
    'docker_logs',
  ];

  let totalPages = $derived(data.totalCount > 0 ? Math.ceil(data.totalCount / pageSize) : 0);
  let currentPath = $derived($page.url.pathname);
  let currentPage = $state(1);
  let currentStatuses = $state<TaskStatusFilter[]>([]);
  let excludedStatuses = $state<TaskStatusFilter[]>([]);
  let currentServiceNames = $state<string[]>([]);
  let excludedServiceNames = $state<string[]>([]);
  let currentNodeIds = $state<string[]>([]);
  let excludedNodeIds = $state<string[]>([]);
  let currentTypes = $state<TaskTypeFilter[]>([]);
  let excludedTypes = $state<TaskTypeFilter[]>([]);

  let filterOpen = $state(false);
  let statusSelectOpen = $state(false);
  let serviceSelectOpen = $state(false);
  let nodeSelectOpen = $state(false);
  let typeSelectOpen = $state(false);
  let statusMode = $state<FilterMode>('include');
  let serviceMode = $state<FilterMode>('include');
  let nodeMode = $state<FilterMode>('include');
  let typeMode = $state<FilterMode>('include');

  let statusTriggerRef = $state<HTMLButtonElement | null>(null);
  let serviceTriggerRef = $state<HTMLButtonElement | null>(null);
  let nodeTriggerRef = $state<HTMLButtonElement | null>(null);
  let typeTriggerRef = $state<HTMLButtonElement | null>(null);

  const activeFilterCount = $derived(
    [
      currentStatuses.length > 0 || excludedStatuses.length > 0,
      currentServiceNames.length > 0 || excludedServiceNames.length > 0,
      currentNodeIds.length > 0 || excludedNodeIds.length > 0,
      currentTypes.length > 0 || excludedTypes.length > 0,
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

  const statusSummary = $derived(selectionSummary(currentStatuses, excludedStatuses, statusOptions.map((status) => ({ value: status, label: statusLabel(status) })), $messages.tasks.filters.allStatuses));
  const serviceSummary = $derived(selectionSummary(currentServiceNames, excludedServiceNames, serviceOptions, $messages.tasks.filters.allServices));
  const nodeSummary = $derived(selectionSummary(currentNodeIds, excludedNodeIds, nodeOptions, $messages.tasks.filters.allNodes));
  const typeSummary = $derived(selectionSummary(currentTypes, excludedTypes, taskTypeOptions.map((type) => ({ value: type, label: typeLabel(type) })), $messages.tasks.filters.allTypes));

  $effect(() => {
    currentPage = data.page;
    currentStatuses = [...data.status] as TaskStatusFilter[];
    excludedStatuses = [...data.excludeStatus] as TaskStatusFilter[];
    currentServiceNames = [...data.serviceName];
    excludedServiceNames = [...data.excludeServiceName];
    currentNodeIds = [...data.nodeId];
    excludedNodeIds = [...data.excludeNodeId];
    currentTypes = [...data.type] as TaskTypeFilter[];
    excludedTypes = [...data.excludeType] as TaskTypeFilter[];
  });

  $effect(() => {
    document.title = `Tasks - Composia`;
  });

  function statusLabel(status: TaskStatusFilter): string {
    return $messages.status[status];
  }

  function typeLabel(type: TaskTypeFilter | string): string {
    if (type === 'rustic_forget') return $messages.tasks.filters.types.rusticForget;
    if (type === 'rustic_prune') return $messages.tasks.filters.types.rusticPrune;
    if (type === 'prune') return $messages.tasks.filters.types.prune;
    if (type === 'dns_update') return $messages.tasks.filters.types.dnsUpdate;
    if (type === 'caddy_sync') return $messages.tasks.filters.types.caddySync;
    if (type === 'caddy_reload') return $messages.tasks.filters.types.caddyReload;
    if (type === 'docker_list') return $messages.tasks.filters.types.dockerList;
    if (type === 'docker_inspect') return $messages.tasks.filters.types.dockerInspect;
    if (type === 'docker_start') return $messages.tasks.filters.types.dockerStart;
    if (type === 'docker_stop') return $messages.tasks.filters.types.dockerStop;
    if (type === 'docker_restart') return $messages.tasks.filters.types.dockerRestart;
    if (type === 'docker_logs') return $messages.tasks.filters.types.dockerLogs;
    return type;
  }

  function selectionSummary(includeValues: string[], excludeValues: string[], options: FilterOption[], emptyLabel = $messages.tasks.filters.allStatuses): string {
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

  function clearModeValues(kind: 'status' | 'service' | 'node' | 'type', mode: FilterMode) {
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
    if (kind === 'type') {
      if (mode === 'include') currentTypes = [];
      if (mode === 'exclude') excludedTypes = [];
    }
  }

  function toggleFilterSelection<T extends string>(kind: 'status' | 'service' | 'node' | 'type', mode: FilterMode, value: T) {
    if (kind === 'status') {
      if (mode === 'include') {
        currentStatuses = toggleValue(currentStatuses as T[], value) as TaskStatusFilter[];
        excludedStatuses = excludedStatuses.filter((entry) => entry !== value) as TaskStatusFilter[];
      } else {
        excludedStatuses = toggleValue(excludedStatuses as T[], value) as TaskStatusFilter[];
        currentStatuses = currentStatuses.filter((entry) => entry !== value) as TaskStatusFilter[];
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
    if (kind === 'type') {
      if (mode === 'include') {
        currentTypes = toggleValue(currentTypes as T[], value) as TaskTypeFilter[];
        excludedTypes = excludedTypes.filter((entry) => entry !== value) as TaskTypeFilter[];
      } else {
        excludedTypes = toggleValue(excludedTypes as T[], value) as TaskTypeFilter[];
        currentTypes = currentTypes.filter((entry) => entry !== value) as TaskTypeFilter[];
      }
    }
  }

  function isSelected(kind: 'status' | 'service' | 'node' | 'type', mode: FilterMode, value: string): boolean {
    if (kind === 'status') return mode === 'include' ? currentStatuses.includes(value as TaskStatusFilter) : excludedStatuses.includes(value as TaskStatusFilter);
    if (kind === 'service') return mode === 'include' ? currentServiceNames.includes(value) : excludedServiceNames.includes(value);
    if (kind === 'node') return mode === 'include' ? currentNodeIds.includes(value) : excludedNodeIds.includes(value);
    return mode === 'include' ? currentTypes.includes(value as TaskTypeFilter) : excludedTypes.includes(value as TaskTypeFilter);
  }

  function closeAndFocus(kind: 'status' | 'service' | 'node' | 'type') {
    if (kind === 'status') {
      statusSelectOpen = false;
    }
    if (kind === 'service') {
      serviceSelectOpen = false;
    }
    if (kind === 'node') {
      nodeSelectOpen = false;
    }
    if (kind === 'type') {
      typeSelectOpen = false;
    }

    tick().then(() => {
      if (kind === 'status') statusTriggerRef?.focus();
      if (kind === 'service') serviceTriggerRef?.focus();
      if (kind === 'node') nodeTriggerRef?.focus();
      if (kind === 'type') typeTriggerRef?.focus();
    });
  }

  function modeLabel(mode: FilterMode): string {
    return mode === 'include' ? $messages.tasks.filters.include : $messages.tasks.filters.exclude;
  }

  function pageUrl(
    page: number,
    statuses: string[],
    excludeStatuses: string[],
    serviceNames: string[],
    excludeServiceNames: string[],
    nodeIds: string[],
    excludeNodeIds: string[],
    types: string[],
    excludeTypes: string[],
  ): string {
    const params = new URLSearchParams();
    if (page > 1) {
      params.set('page', page.toString());
    }
    for (const status of statuses) {
      params.append('status', status);
    }
    for (const status of excludeStatuses) {
      params.append('excludeStatus', status);
    }
    for (const serviceName of serviceNames) {
      params.append('serviceName', serviceName);
    }
    for (const serviceName of excludeServiceNames) {
      params.append('excludeServiceName', serviceName);
    }
    for (const nodeId of nodeIds) {
      params.append('nodeId', nodeId);
    }
    for (const nodeId of excludeNodeIds) {
      params.append('excludeNodeId', nodeId);
    }
    for (const type of types) {
      params.append('type', type);
    }
    for (const type of excludeTypes) {
      params.append('excludeType', type);
    }

    const query = params.toString();
    return query ? `${currentPath}?${query}` : currentPath;
  }

  function applyFilters() {
    currentPage = 1;
    filterOpen = false;
    void goto(pageUrl(1, currentStatuses, excludedStatuses, currentServiceNames, excludedServiceNames, currentNodeIds, excludedNodeIds, currentTypes, excludedTypes));
  }

  function resetFilters() {
    currentStatuses = [];
    excludedStatuses = [];
    currentServiceNames = [];
    excludedServiceNames = [];
    currentNodeIds = [];
    excludedNodeIds = [];
    currentTypes = [];
    excludedTypes = [...defaultExcludedTypes];
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
      && JSON.stringify(currentTypes) === JSON.stringify(data.type)
      && JSON.stringify(excludedTypes) === JSON.stringify(data.excludeType)
    ) {
      return;
    }

    if (currentPage !== data.page) {
      void goto(pageUrl(currentPage, currentStatuses, excludedStatuses, currentServiceNames, excludedServiceNames, currentNodeIds, excludedNodeIds, currentTypes, excludedTypes));
    }
  });
</script>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title">{$messages.tasks.taskHistory}</CardTitle>
        </div>
        <div class="flex items-center gap-2">
          <Popover.Root bind:open={filterOpen}>
            <Popover.Trigger class="inline-flex">
              {#snippet child({ props })}
                <Button type="button" variant="outline" class="gap-2" {...props}>
                  <FilterIcon class="size-4" />
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
                      <ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
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
                              <CheckIcon class={cn('ml-auto size-4', (statusMode === 'include' ? currentStatuses.length > 0 : excludedStatuses.length > 0) && 'text-transparent')} />
                            </Command.Item>
                            {#each statusOptions as status}
                              <Command.Item value={status} onSelect={() => { toggleFilterSelection('status', statusMode, status); }}>
                                <span>{statusLabel(status)}</span>
                                <CheckIcon class={cn('ml-auto size-4', !isSelected('status', statusMode, status) && 'text-transparent')} />
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
                      <ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
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
                              <CheckIcon class={cn('ml-auto size-4', (serviceMode === 'include' ? currentServiceNames.length > 0 : excludedServiceNames.length > 0) && 'text-transparent')} />
                            </Command.Item>
                            {#each serviceOptions as service (service.value)}
                              <Command.Item value={service.value} onSelect={() => { toggleFilterSelection('service', serviceMode, service.value); }}>
                                <span class="truncate">{service.label}</span>
                                <CheckIcon class={cn('ml-auto size-4', !isSelected('service', serviceMode, service.value) && 'text-transparent')} />
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
                      <ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
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
                              <CheckIcon class={cn('ml-auto size-4', (nodeMode === 'include' ? currentNodeIds.length > 0 : excludedNodeIds.length > 0) && 'text-transparent')} />
                            </Command.Item>
                            {#each nodeOptions as node (node.value)}
                              <Command.Item value={`${node.label} ${node.secondary ?? ''}`} onSelect={() => { toggleFilterSelection('node', nodeMode, node.value); }}>
                                <div class="min-w-0">
                                  <div class="truncate">{node.label}</div>
                                  {#if node.secondary}
                                    <div class="truncate text-xs text-muted-foreground">{node.secondary}</div>
                                  {/if}
                                </div>
                                <CheckIcon class={cn('ml-auto size-4', !isSelected('node', nodeMode, node.value) && 'text-transparent')} />
                              </Command.Item>
                            {/each}
                          </Command.Group>
                        </Command.List>
                      </Command.Root>
                    </Popover.Content>
                  </Popover.Root>
                </div>

                <div class="space-y-2">
                  <div class="text-sm text-muted-foreground">{$messages.tasks.filters.type}</div>
                  <Popover.Root bind:open={typeSelectOpen}>
                    <Popover.Trigger bind:ref={typeTriggerRef} class={buttonVariants({ variant: 'outline', class: 'w-full justify-between font-normal' })}>
                      <span class="truncate">{typeSummary}</span>
                      <ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
                    </Popover.Trigger>
                    <Popover.Content class="w-[min(92vw,24rem)] p-0" align="start">
                      <div class="flex border-b p-1 gap-1">
                        <Button type="button" size="sm" variant={typeMode === 'include' ? 'default' : 'ghost'} onclick={() => { typeMode = 'include'; }}>{modeLabel('include')}</Button>
                        <Button type="button" size="sm" variant={typeMode === 'exclude' ? 'default' : 'ghost'} onclick={() => { typeMode = 'exclude'; }}>{modeLabel('exclude')}</Button>
                      </div>
                      <Command.Root>
                        <Command.Input placeholder={$messages.tasks.filters.searchTypePlaceholder} />
                        <Command.List>
                          <Command.Empty>{$messages.tasks.filters.noTypesFound}</Command.Empty>
                          <Command.Group>
                            <Command.Item value="__all__" onSelect={() => { clearModeValues('type', typeMode); closeAndFocus('type'); }}>
                              <span>{typeMode === 'include' ? $messages.tasks.filters.clearIncluded : $messages.tasks.filters.clearExcluded}</span>
                              <CheckIcon class={cn('ml-auto size-4', (typeMode === 'include' ? currentTypes.length > 0 : excludedTypes.length > 0) && 'text-transparent')} />
                            </Command.Item>
                            {#each taskTypeOptions as type}
                              <Command.Item value={`${typeLabel(type)} ${type}`} onSelect={() => { toggleFilterSelection('type', typeMode, type); }}>
                                <span class="truncate">{typeLabel(type)}</span>
                                <CheckIcon class={cn('ml-auto size-4', !isSelected('type', typeMode, type) && 'text-transparent')} />
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
      <div class="space-y-3">
        {#if data.tasks.length}
          {#each data.tasks as task}
            <TaskItem {task} showNode />
          {/each}
        {:else}
          <div class="empty-state">
            {#if data.status.length === 0 && data.excludeStatus.length === 0 && data.serviceName.length === 0 && data.excludeServiceName.length === 0 && data.nodeId.length === 0 && data.excludeNodeId.length === 0 && data.type.length === 0 && JSON.stringify(data.excludeType) === JSON.stringify(defaultExcludedTypes)}
              {$messages.tasks.noTasks}
            {:else}
              {$messages.tasks.noTasksForFilter}
            {/if}
          </div>
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
