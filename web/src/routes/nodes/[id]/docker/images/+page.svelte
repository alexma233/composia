<script lang="ts">
  import type { PageData } from './$types';

  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  export let data: PageData;

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }
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
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            ← Back to node
          </a>
        </div>
      </CardHeader>
      <CardContent>
        {#if data.images && data.images.length > 0}
          <div class="space-y-3">
            {#each data.images as image}
              <div class="rounded-lg border border-border/70 bg-background/80 p-4">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="space-y-1">
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-sm font-medium">
                        {image.repoTags && image.repoTags.length > 0 ? image.repoTags[0] : image.id}
                      </span>
                      <button
                        on:click={() => copyToClipboard(image.id)}
                        class="text-xs text-muted-foreground hover:text-foreground"
                        title="Copy ID"
                      >
                        📋
                      </button>
                    </div>
                    <div class="text-xs text-muted-foreground">
                      Size: {formatSize(image.size)}
                    </div>
                    {#if image.created}
                      <div class="text-xs text-muted-foreground">
                        Created: {image.created}
                      </div>
                    {/if}
                  </div>
                  <a
                    href="/nodes/{data.nodeId}/docker/images/{encodeURIComponent(image.id)}"
                    class="text-sm text-muted-foreground hover:text-foreground hover:underline"
                  >
                    Inspect →
                  </a>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state">No images found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
