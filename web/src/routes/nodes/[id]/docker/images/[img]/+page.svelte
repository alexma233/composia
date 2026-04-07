<script lang="ts">
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge } from '$lib/components/ui/badge';
  import { formatBytes, parseJsonList } from '$lib/presenters';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let imageData = $state<any>(null);
  let parseError = $state<string | null>(null);

  $effect(() => {
    if (!data.rawJson) {
      imageData = null;
      parseError = null;
      return;
    }

    try {
      imageData = parseJsonList(data.rawJson);
      parseError = null;
    } catch (error) {
      parseError = error instanceof Error ? error.message : 'Failed to parse image data';
      imageData = null;
    }
  });

  function formatDate(timestamp: string): string {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleString();
  }
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">
              {#if imageData}
                {imageData.RepoTags?.[0]?.split(':')[0] || data.imageId.substring(0, 12)}
              {:else}
                Image
              {/if}
            </CardTitle>
            <CardDescription class="page-description">
              {#if imageData}
                <code class="text-xs bg-muted px-1 py-0.5 rounded">{imageData.Id?.substring(0, 19)}</code>
              {:else}
                {data.imageId}
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}/docker/images" class="text-sm text-muted-foreground hover:underline">
            Back to images
          </a>
        </div>
      </CardHeader>

      <CardContent>
        {#if data.error}
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            {data.error}
          </div>
        {:else if parseError}
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            Failed to parse image data: {parseError}
          </div>
        {:else if imageData}
          <Tabs value="info" class="w-full">
            <TabsList class="mb-4">
              <TabsTrigger value="info">Info</TabsTrigger>
              <TabsTrigger value="layers">Layers</TabsTrigger>
              <TabsTrigger value="raw">JSON</TabsTrigger>
            </TabsList>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Identity</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Image ID</span>
                      <code class="text-xs bg-muted px-1 py-0.5 rounded">{imageData.Id?.substring(0, 19)}</code>
                    </div>
                    {#if imageData.RepoTags && imageData.RepoTags.length > 0}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Tags</span>
                        <div class="flex flex-wrap gap-1">
                          {#each imageData.RepoTags.slice(0, 5) as tag}
                            <Badge variant="outline" class="text-xs">{tag}</Badge>
                          {/each}
                          {#if imageData.RepoTags.length > 5}
                            <span class="text-xs text-muted-foreground">+{imageData.RepoTags.length - 5}</span>
                          {/if}
                        </div>
                      </div>
                    {/if}
                    {#if imageData.RepoDigests && imageData.RepoDigests.length > 0}
                      <div class="flex flex-col gap-1">
                        <span class="text-muted-foreground">Digests</span>
                        <div class="flex flex-wrap gap-1">
                          {#each imageData.RepoDigests.slice(0, 3) as digest}
                            <code class="text-xs bg-muted px-1 py-0.5 rounded truncate max-w-[200px]" title={digest}>
                              {digest.substring(0, 30)}...
                            </code>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Details</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Size</span>
                      <Badge variant="secondary">{formatBytes(imageData.Size || imageData.VirtualSize)}</Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Architecture</span>
                      <span>{imageData.Architecture || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">OS</span>
                      <span>{imageData.Os || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Created</span>
                      <span>{formatDate(imageData.Created)}</span>
                    </div>
                    {#if imageData.Author}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Author</span>
                        <span>{imageData.Author}</span>
                      </div>
                    {/if}
                  </CardContent>
                </Card>
              </div>

              {#if imageData.Config?.Env && imageData.Config.Env.length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Environment Variables</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each imageData.Config.Env as env}
                        {@const [key, ...valueParts] = env.split('=')}
                        {@const value = valueParts.join('=')}
                        <div class="flex gap-2 text-sm font-mono text-xs border-b border-border/50 last:border-0 py-1.5">
                          <span class="text-foreground font-medium shrink-0">{key}=</span>
                          <span class="text-muted-foreground break-all">{value}</span>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {/if}
            </TabsContent>

            <TabsContent value="layers" class="space-y-4">
              {#if imageData.RootFS?.Layers && imageData.RootFS.Layers.length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Layers ({imageData.RootFS.Layers.length})</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-2">
                      {#each imageData.RootFS.Layers as layer, i}
                        <div class="flex items-center gap-3 py-2 border-b border-border/50 last:border-0">
                          <span class="text-xs text-muted-foreground w-6">{i + 1}</span>
                          <code class="text-xs bg-muted px-1 py-0.5 rounded flex-1 truncate" title={layer}>
                            {layer.substring(0, 19)}
                          </code>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {:else}
                <div class="text-sm text-muted-foreground">No layer information available</div>
              {/if}
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Raw JSON</CardTitle>
                  <CardDescription>Full image inspection data in JSON format</CardDescription>
                </CardHeader>
                <CardContent>
                  <pre class="text-xs font-mono overflow-auto whitespace-pre-wrap break-all bg-background/80 p-4 rounded-lg border border-border/70 max-h-[600px]">{JSON.stringify(imageData, null, 2)}</pre>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        {:else}
          <div class="text-sm text-muted-foreground">Loading...</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
