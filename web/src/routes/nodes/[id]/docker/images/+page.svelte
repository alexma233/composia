<script lang="ts">
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge, type Variant } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';

  export let data: PageData;

  let searchQuery = '';
  let sortField: 'name' | 'size' | 'created' = 'name';
  let sortDirection: 'asc' | 'desc' = 'asc';

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function formatShortId(id: string): string {
    return id.substring(0, 12);
  }

  function formatSize(bytes: number): string {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  function formatRelativeTime(timestamp: string): string {
    if (!timestamp) return '-';
    
    // Handle Docker's "2006-01-02 15:04:05 +0700 MST" format
    let date: Date;
    if (timestamp.includes(' +') || timestamp.includes(' -')) {
      const cleaned = timestamp.replace(/\s+[+-]\d{4}\s+\w+$/, '');
      const parts = cleaned.split(' ');
      if (parts.length === 2) {
        date = new Date(parts[0] + 'T' + parts[1]);
      } else {
        date = new Date(cleaned);
      }
    } else {
      date = new Date(timestamp);
    }
    
    if (isNaN(date.getTime())) return timestamp;
    const now = new Date();
    const diff = Math.floor((now.getTime() - date.getTime()) / 1000);

    if (diff < 0) return 'just now';
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  }

  function handleSort(field: typeof sortField) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field;
      sortDirection = 'asc';
    }
  }

  const SortIcon = (field: typeof sortField) => {
    if (sortField !== field) {
      return `<svg class="w-3 h-3 text-muted-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M7 15l5 5 5-5M7 9l5-5 5 5"/>
      </svg>`;
    }
    if (sortDirection === 'asc') {
      return `<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 5v14M5 12l7-7 7 7"/>
      </svg>`;
    }
    return `<svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M12 19V5M5 12l7 7 7-7"/>
    </svg>`;
  };

  $: filteredImages = (data.images || []).filter((i) => {
    const query = searchQuery.toLowerCase();
    const tags = i.repoTags || [];
    return (
      tags.some((t) => t.toLowerCase().includes(query)) ||
      i.id.toLowerCase().includes(query) ||
      (i.architecture || '').toLowerCase().includes(query)
    );
  });

  $: sortedImages = [...filteredImages].sort((a, b) => {
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
  });
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">Images</CardTitle>
            <CardDescription class="page-description">
              Docker images on {data.nodeId}
              {#if data.images}
                <Badge variant="secondary" class="ml-2">{data.images.length}</Badge>
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            Back to node
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <svg
              class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.3-4.3" />
            </svg>
            <Input
              type="text"
              placeholder="Search images..."
              class="pl-9"
              bind:value={searchQuery}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" on:click={() => (searchQuery = '')}>
              Clear
            </Button>
          {/if}
        </div>
      </CardHeader>
      <CardContent>
        {#if data.error}
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            {data.error}
          </div>
        {:else if sortedImages.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="w-[40%]">
                  <button class="flex items-center gap-1 hover:text-foreground" on:click={() => handleSort('name')}>
                    Image
                    {@html SortIcon('name')}
                  </button>
                </TableHead>
                <TableHead class="w-[15%]">
                  <button class="flex items-center gap-1 hover:text-foreground" on:click={() => handleSort('size')}>
                    Size
                    {@html SortIcon('size')}
                  </button>
                </TableHead>
                <TableHead class="w-[20%]">Architecture</TableHead>
                <TableHead class="w-[15%]">Usage</TableHead>
                <TableHead class="w-[15%]">
                  <button class="flex items-center gap-1 hover:text-foreground" on:click={() => handleSort('created')}>
                    Created
                    {@html SortIcon('created')}
                  </button>
                </TableHead>
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
                        <button
                          on:click={() => copyToClipboard(image.id)}
                          class="text-muted-foreground hover:text-foreground"
                          title="Copy full ID"
                        >
                          <svg
                            class="h-3.5 w-3.5"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          >
                            <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
                            <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
                          </svg>
                        </button>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <span class="text-sm">{formatSize(image.size)}</span>
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
                      <Badge variant="success">{image.containersCount} container{image.containersCount > 1 ? 's' : ''}</Badge>
                    {:else}
                      <Badge variant="secondary">Unused</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={image.created}>
                      {formatRelativeTime(image.created)}
                    </div>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if filteredImages.length !== data.images?.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              Showing {filteredImages.length} of {data.images?.length} images
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
