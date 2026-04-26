<script lang="ts">
  import type { PageData } from './$types';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge } from '$lib/components/ui/badge';
  import { formatBytes, formatTimestamp, parseJsonList } from "$lib/presenters";
  import { messages } from '$lib/i18n';

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
</script>

<svelte:head>
  <title>{$messages.docker.images.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="page-header">
          <div class="page-heading">
            <CardTitle class="page-title">
              {#if imageData}
                {imageData.RepoTags?.[0]?.split(':')[0] || data.imageId.substring(0, 12)}
              {:else}
                {$messages.docker.images.title}
              {/if}
            </CardTitle>
            <p class="page-description">
              {#if imageData}
                <code class="text-xs bg-muted px-1 py-0.5 rounded">{imageData.Id?.substring(0, 19)}</code>
              {:else}
                {data.imageId}
              {/if}
            </p>
          </div>
          <a href="/nodes/{data.nodeId}/docker/images" class="text-sm text-muted-foreground transition-colors hover:text-foreground">
            {$messages.docker.images.backToImages}
          </a>
        </div>
      </CardHeader>

      <CardContent>
        {#if data.error}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
            <AlertDescription>{data.error}</AlertDescription>
          </Alert>
        {:else if parseError}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.parseFailed}</AlertTitle>
            <AlertDescription>{$messages.error.parseFailed}: {parseError}</AlertDescription>
          </Alert>
        {:else if imageData}
          <Tabs value="info" class="w-full">
            <div class="mb-4 overflow-x-auto pb-1 scrollbar-none">
              <TabsList class="min-w-max">
                <TabsTrigger value="info">{$messages.docker.containers.info}</TabsTrigger>
                <TabsTrigger value="layers">{$messages.docker.images.layers}</TabsTrigger>
                <TabsTrigger value="raw">{$messages.docker.containers.json}</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.images.identity}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.images.id}</span>
                      <code class="text-xs bg-muted px-1 py-0.5 rounded">{imageData.Id?.substring(0, 19)}</code>
                    </div>
                    {#if imageData.RepoTags && imageData.RepoTags.length > 0}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.images.tags}</span>
                        <div class="flex flex-wrap gap-1 sm:max-w-[20rem] sm:justify-end">
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
                        <span class="text-muted-foreground">{$messages.docker.images.digests}</span>
                        <div class="flex flex-wrap gap-1">
                          {#each imageData.RepoDigests.slice(0, 3) as digest}
                            <code class="max-w-full break-all rounded bg-muted px-1 py-0.5 text-xs sm:max-w-[200px] sm:truncate" title={digest}>
                              {digest}
                            </code>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.images.details}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.images.size}</span>
                      <Badge variant="secondary">{formatBytes(imageData.Size || imageData.VirtualSize)}</Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.images.architecture}</span>
                      <span class="break-all sm:text-right">{imageData.Architecture || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.images.os}</span>
                      <span class="break-all sm:text-right">{imageData.Os || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.common.created}</span>
                      <span class="sm:text-right">{formatTimestamp(imageData.Created)}</span>
                    </div>
                    {#if imageData.Author}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.images.author}</span>
                        <span class="break-all sm:text-right">{imageData.Author}</span>
                      </div>
                    {/if}
                  </CardContent>
                </Card>
              </div>

              {#if imageData.Config?.Env && imageData.Config.Env.length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.images.environmentVariables}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each imageData.Config.Env as env}
                        {@const [key, ...valueParts] = env.split('=')}
                        {@const value = valueParts.join('=')}
                        <div class="flex flex-col gap-1 border-b border-border/50 py-1.5 font-mono text-xs last:border-0 sm:flex-row sm:gap-2">
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
                    <CardTitle class="text-base">{$messages.docker.images.layers} ({imageData.RootFS.Layers.length})</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-2">
                      {#each imageData.RootFS.Layers as layer, i}
                        <div class="flex flex-col gap-2 border-b border-border/50 py-2 last:border-0 sm:flex-row sm:items-center sm:gap-3">
                          <span class="text-xs text-muted-foreground w-6">{i + 1}</span>
                          <code class="flex-1 break-all rounded bg-muted px-1 py-0.5 text-xs sm:truncate" title={layer}>
                            {layer}
                          </code>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {:else}
                <div class="text-sm text-muted-foreground">{$messages.docker.images.noLayerInfo}</div>
              {/if}
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">{$messages.docker.images.rawJson}</CardTitle>
                  <p class="text-sm text-muted-foreground">{$messages.docker.images.rawJsonDescription}</p>
                </CardHeader>
                <CardContent>
                  <pre class="code-surface max-h-[360px] overflow-auto break-all sm:max-h-[600px]">{JSON.stringify(imageData, null, 2)}</pre>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        {:else}
          <div class="text-sm text-muted-foreground">{$messages.common.loading}...</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
