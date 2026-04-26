<script lang="ts">
  import type { PageData } from './$types';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge } from '$lib/components/ui/badge';
  import { messages } from '$lib/i18n';

  import { formatBytes, formatTimestamp } from '$lib/presenters';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let volumeData = $state<any>(null);
  let parseError = $state<string | null>(null);
  let activeTab = $state('info');

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
      parseError = error instanceof Error ? error.message : $messages.error.parseFailed;
      volumeData = null;
    }
  });
</script>

<svelte:head>
  <title>{$messages.docker.volumes.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="page-header">
          <div class="page-heading">
            <CardTitle class="page-title">
              {#if volumeData}
                {volumeData.Name || data.volumeName}
              {:else}
                {$messages.docker.volumes.title}
              {/if}
            </CardTitle>
            <p class="page-description">
              {#if volumeData}
                <code class="text-xs bg-muted px-1 py-0.5 rounded">{volumeData.Name || data.volumeName}</code>
                <Badge variant="outline" class="ml-2">{volumeData.Driver || $messages.common.local}</Badge>
              {:else}
                {data.volumeName}
              {/if}
            </p>
          </div>
          <a href="/nodes/{data.nodeId}/docker/volumes" class="text-sm text-muted-foreground transition-colors hover:text-foreground">
            {$messages.docker.volumes.backToVolumes}
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
        {:else if volumeData}
          <Tabs bind:value={activeTab} class="w-full">
            <div class="mb-4 overflow-x-auto pb-1 scrollbar-none">
              <TabsList class="min-w-max">
                <TabsTrigger value="info">{$messages.docker.containers.info}</TabsTrigger>
                <TabsTrigger value="usage">{$messages.docker.volumes.usage}</TabsTrigger>
                <TabsTrigger value="raw">{$messages.docker.containers.json}</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.volumes.details}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.volumes.driver}</span>
                      <Badge variant="outline">{volumeData.Driver || 'local'}</Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.volumes.scope}</span>
                        <span class="break-all sm:text-right">{volumeData.Scope || $messages.common.local}</span>
                    </div>
                    {#if volumeData.Options && Object.keys(volumeData.Options).length > 0}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.volumes.options}</span>
                        <span class="text-xs sm:text-right">{Object.keys(volumeData.Options).length} {$messages.docker.volumes.options}</span>
                      </div>
                    {/if}
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.volumes.created}</span>
                      <span class="sm:text-right">{formatTimestamp(volumeData.CreatedAt)}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.volumes.storage}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    {#if volumeData.UsageData?.Size}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.volumes.size}</span>
                        <Badge variant="secondary">{formatBytes(volumeData.UsageData.Size)}</Badge>
                      </div>
                    {:else}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.volumes.size}</span>
                        <span class="text-muted-foreground">-</span>
                      </div>
                    {/if}
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.volumes.mountpoint}</span>
                      <code class="max-w-full break-all rounded bg-muted px-1 py-0.5 text-xs sm:max-w-[20rem] sm:text-right" title={volumeData.Mountpoint}>
                        {volumeData.Mountpoint}
                      </code>
                    </div>
                  </CardContent>
                </Card>
              </div>

              {#if volumeData.Labels && Object.keys(volumeData.Labels).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.volumes.labels}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each Object.entries(volumeData.Labels) as [key, value]}
                        <div class="flex flex-col gap-1 border-b border-border/50 py-1.5 text-sm last:border-0 sm:flex-row sm:gap-2">
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
                    <CardTitle class="text-base">{$messages.docker.volumes.options}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each Object.entries(volumeData.Options) as [key, value]}
                        <div class="flex flex-col gap-1 border-b border-border/50 py-1.5 text-sm last:border-0 sm:flex-row sm:gap-2">
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
                    <CardTitle class="text-base">{$messages.docker.volumes.usageStatistics}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.volumes.size}</span>
                      <Badge variant="secondary">{formatBytes(volumeData.UsageData.Size)}</Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.volumes.refCount}</span>
                      <Badge variant={volumeData.UsageData.RefCount > 0 ? 'default' : 'secondary'}>
                        {volumeData.UsageData.RefCount === 1
                          ? $messages.docker.volumes.containerCount.replace('{count}', String(volumeData.UsageData.RefCount))
                          : $messages.docker.volumes.containersCount.replace('{count}', String(volumeData.UsageData.RefCount))}
                      </Badge>
                    </div>
                  </CardContent>
                </Card>
              {:else}
                <div class="text-sm text-muted-foreground">
                  {$messages.docker.volumes.usageNotAvailable}
                </div>
              {/if}
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">{$messages.docker.volumes.rawJson}</CardTitle>
                  <CardDescription>{$messages.docker.volumes.rawJsonDescription}</CardDescription>
                </CardHeader>
                <CardContent>
                  <pre class="code-surface max-h-[360px] overflow-auto break-all sm:max-h-[600px]">{JSON.stringify(volumeData, null, 2)}</pre>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        {:else}
          <div class="text-sm text-muted-foreground">{$messages.common.loadingWithDots}</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
