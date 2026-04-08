<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import { formatBytes, formatDockerTimestamp, formatShortId } from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
  import { Alert, AlertDescription } from '$lib/components/ui/alert';

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

  let searchQuery = $state('');
  let sortField = $state<'name' | 'size' | 'created'>('name');
  let sortDirection = $state<'asc' | 'desc'>('asc');
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let images = $state<DockerImageSummary[]>([]);

  $effect(() => {
    loading = !data.ready;
    loadError = data.error ?? null;
    images = data.images || [];
  });

  async function refreshImages() {
    if (!data.ready) {
      loading = false;
      return;
    }

    loading = true;
    loadError = null;

    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/images`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error || 'Failed to load images');
      }
      images = payload.images ?? [];
    } catch (error) {
      loadError = error instanceof Error ? error.message : 'Failed to load images';
      images = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void refreshImages();
  });

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as typeof sortField;
      sortDirection = 'asc';
    }
  }

  let filteredImages = $derived(images.filter((i) => {
    const query = searchQuery.toLowerCase();
    const tags = i.repoTags || [];
    return (
      tags.some((t) => t.toLowerCase().includes(query)) ||
      i.id.toLowerCase().includes(query) ||
      (i.architecture || '').toLowerCase().includes(query)
    );
  }));

  let sortedImages = $derived([...filteredImages].sort((a, b) => {
    let comparison = 0;
    const aTags = a.repoTags || [];
    const bTags = b.repoTags || [];
    switch (sortField) {
      case 'name':
        comparison = (aTags[0] || a.id).localeCompare(bTags[0] || b.id);
        break;
      case 'size':
        comparison = (a.size || 0) - (b.size || 0);
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
            <CardTitle class="page-title">Images</CardTitle>
            <CardDescription class="page-description">
              Docker images on {data.nodeId}
              {#if !loading}
                <Badge variant="secondary" class="ml-2">{images.length}</Badge>
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            Back to node
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <SearchIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search images..."
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={() => (searchQuery = '')}>
              Clear
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void refreshImages()} disabled={loading || !data.ready}>
            {#if loading}Loading...{:else}Refresh{/if}
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
              <span>Loading images...</span>
            </div>
          </div>
        {:else if sortedImages.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label="Image" {sortField} {sortDirection} onSort={handleSort} class="w-[40%]" />
                <SortableTableHead field="size" label="Size" {sortField} {sortDirection} onSort={handleSort} class="w-[15%]" />
                <TableHead class="w-[20%]">Architecture</TableHead>
                <TableHead class="w-[15%]">Usage</TableHead>
                <SortableTableHead field="created" label="Created" {sortField} {sortDirection} onSort={handleSort} class="w-[15%]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each sortedImages as image}
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
                          <div class="text-xs text-muted-foreground">+{image.repoTags.length - 1} more tags</div>
                        {/if}
                      {:else if image.isDangling}
                        <a
                          href="/nodes/{data.nodeId}/docker/images/{encodeURIComponent(image.id)}"
                          class="font-medium text-muted-foreground hover:underline"
                        >&lt;none&gt;</a>
                        <Badge variant="secondary" class="text-xs">Dangling</Badge>
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
                        <CopyButton text={image.id} label="Copy full ID" />
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
                      <Badge variant="default">{image.containersCount} container{image.containersCount > 1 ? 's' : ''}</Badge>
                    {:else}
                      <Badge variant="secondary">Unused</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                      <div class="text-sm text-muted-foreground" title={image.created}>
                        {formatDockerTimestamp(image.created)}
                      </div>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredImages.length !== images.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredImages.length} of {images.length} images
            </div>
          {/if}
        {:else if searchQuery}
          <div class="empty-state">
            No images matching "{searchQuery}".
          </div>
        {:else}
          <div class="empty-state">No images found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
