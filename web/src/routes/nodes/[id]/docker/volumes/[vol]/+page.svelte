<script lang="ts">
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge } from '$lib/components/ui/badge';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let volumeData = $state<any>(null);
  let parseError = $state<string | null>(null);

  $effect(() => {
    if (!data.rawJson) {
      volumeData = null;
      parseError = null;
      return;
    }

    try {
      const parsed = JSON.parse(data.rawJson);
      volumeData = Array.isArray(parsed) ? parsed[0] : parsed;
      parseError = null;
    } catch (error) {
      parseError = error instanceof Error ? error.message : 'Failed to parse volume data';
      volumeData = null;
    }
  });

  function formatSize(bytes: number): string {
    if (!bytes) return '-';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

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
              {#if volumeData}
                {volumeData.Name || data.volumeName}
              {:else}
                Volume
              {/if}
            </CardTitle>
            <CardDescription class="page-description">
              {#if volumeData}
                <Badge variant="outline">{volumeData.Driver || 'local'}</Badge>
              {:else}
                {data.volumeName}
              {/if}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}/docker/volumes" class="text-sm text-muted-foreground hover:underline">
            Back to volumes
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
            Failed to parse volume data: {parseError}
          </div>
        {:else if volumeData}
          <Tabs value="info" class="w-full">
            <TabsList class="mb-4">
              <TabsTrigger value="info">Info</TabsTrigger>
              <TabsTrigger value="usage">Usage</TabsTrigger>
              <TabsTrigger value="raw">JSON</TabsTrigger>
            </TabsList>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Details</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Driver</span>
                      <Badge variant="outline">{volumeData.Driver || 'local'}</Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Scope</span>
                      <span>{volumeData.Scope || 'local'}</span>
                    </div>
                    {#if volumeData.Options && Object.keys(volumeData.Options).length > 0}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Options</span>
                        <span class="text-xs">{Object.keys(volumeData.Options).length} options</span>
                      </div>
                    {/if}
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Created</span>
                      <span>{formatDate(volumeData.CreatedAt)}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Storage</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    {#if volumeData.UsageData?.Size}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Size</span>
                        <Badge variant="secondary">{formatSize(volumeData.UsageData.Size)}</Badge>
                      </div>
                    {:else}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Size</span>
                        <span class="text-muted-foreground">-</span>
                      </div>
                    {/if}
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Mount Point</span>
                      <code class="text-xs bg-muted px-1 py-0.5 rounded truncate max-w-[200px]" title={volumeData.Mountpoint}>
                        {volumeData.Mountpoint}
                      </code>
                    </div>
                  </CardContent>
                </Card>
              </div>

              {#if volumeData.Labels && Object.keys(volumeData.Labels).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Labels</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each Object.entries(volumeData.Labels) as [key, value]}
                        <div class="flex gap-2 text-sm border-b border-border/50 last:border-0 py-1.5">
                          <code class="text-xs bg-muted px-1 py-0.5 rounded shrink-0">{key}</code>
                          <span class="text-muted-foreground break-all">{value}</span>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {/if}

              {#if volumeData.Options && Object.keys(volumeData.Options).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Options</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each Object.entries(volumeData.Options) as [key, value]}
                        <div class="flex gap-2 text-sm border-b border-border/50 last:border-0 py-1.5">
                          <code class="text-xs bg-muted px-1 py-0.5 rounded shrink-0">{key}</code>
                          <span class="text-muted-foreground break-all">{value}</span>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {/if}
            </TabsContent>

            <TabsContent value="usage" class="space-y-4">
              {#if volumeData.UsageData}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Usage Statistics</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Size</span>
                      <Badge variant="secondary">{formatSize(volumeData.UsageData.Size)}</Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Ref Count</span>
                      <Badge variant={volumeData.UsageData.RefCount > 0 ? 'success' : 'secondary'}>
                        {volumeData.UsageData.RefCount} container{volumeData.UsageData.RefCount !== 1 ? 's' : ''}
                      </Badge>
                    </div>
                  </CardContent>
                </Card>
              {:else}
                <div class="text-sm text-muted-foreground">
                  Usage statistics are not available for this volume. This may occur when the volume is not in use or when the Docker version does not support volume usage reporting.
                </div>
              {/if}
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Raw JSON</CardTitle>
                  <CardDescription>Full volume inspection data in JSON format</CardDescription>
                </CardHeader>
                <CardContent>
                  <pre class="text-xs font-mono overflow-auto whitespace-pre-wrap break-all bg-background/80 p-4 rounded-lg border border-border/70 max-h-[600px]">{JSON.stringify(volumeData, null, 2)}</pre>
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
