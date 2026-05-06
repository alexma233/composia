<script lang="ts">
  import type { PageData } from './$types';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge } from '$lib/components/ui/badge';
  import { messages } from '$lib/i18n';

  import { formatTimestamp } from '$lib/presenters';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let networkData = $state<any>(null);
  let parseError = $state<string | null>(null);
  let activeTab = $state('info');

  $effect(() => {
    if (!data.rawJson) {
      networkData = null;
      parseError = null;
      return;
    }

    try {
      const parsed = JSON.parse(data.rawJson);
      networkData = Array.isArray(parsed) ? parsed[0] : parsed;
      parseError = null;
    } catch (error) {
      parseError = error instanceof Error ? error.message : $messages.error.parseFailed;
      networkData = null;
    }
  });

  function getDriverColor(driver: string): 'default' | 'outline' | 'secondary' {
    const d = driver?.toLowerCase() || '';
    if (d === 'bridge' || d === 'host') return 'default';
    if (d === 'overlay') return 'outline';
    if (d === 'macvlan') return 'secondary';
    return 'outline';
  }
</script>

<svelte:head>
  <title>{$messages.docker.networks.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="page-header">
          <div class="page-heading">
            <CardTitle class="page-title" level="1">
              {#if networkData}
                {networkData.Name || data.networkId.substring(0, 12)}
              {:else}
                {$messages.docker.networks.title}
              {/if}
            </CardTitle>
            <p class="page-description">
              {#if networkData}
                <code class="text-xs bg-muted px-1 py-0.5 rounded">{networkData.Id?.substring(0, 19)}</code>
              {:else}
                {data.networkId}
              {/if}
            </p>
          </div>
          <div class="flex items-center gap-2">
            {#if networkData}
              <Badge variant={networkData.Scope === 'local' ? 'secondary' : 'outline'}>
                {networkData.Scope}
              </Badge>
            {/if}
            <a href="/nodes/{data.nodeId}/docker/networks" class="text-sm text-muted-foreground transition-colors hover:text-foreground">
              {$messages.docker.networks.backToNetworks}
            </a>
          </div>
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
        {:else if networkData}
          <Tabs bind:value={activeTab} class="w-full">
            <div class="mb-4 overflow-x-auto pb-1 scrollbar-none">
              <TabsList class="min-w-max">
                <TabsTrigger value="info">{$messages.docker.containers.info}</TabsTrigger>
                <TabsTrigger value="containers">{$messages.docker.networks.containers}</TabsTrigger>
                <TabsTrigger value="raw">{$messages.docker.containers.json}</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-3">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base" level="3">{$messages.docker.networks.configuration}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.driver}</span>
                      <Badge variant={getDriverColor(networkData.Driver)}>{networkData.Driver}</Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.scope}</span>
                      <span class="break-all sm:text-right">{networkData.Scope}</span>
                    </div>
                    {#if networkData.Labels && Object.keys(networkData.Labels).length > 0 && networkData.Labels['com.docker.compose.project']}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.networks.composeProject}</span>
                        <span class="break-all sm:text-right">{networkData.Labels['com.docker.compose.project']}</span>
                      </div>
                    {/if}
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.common.created}</span>
                      <span class="sm:text-right">{formatTimestamp(networkData.Created)}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base" level="3">{$messages.docker.networks.networkSettings}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    {#if networkData.IPAM?.Config && networkData.IPAM.Config.length > 0}
                       <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                         <span class="text-muted-foreground">{$messages.docker.networks.subnet}</span>
                         <code class="break-all rounded bg-muted px-1 py-0.5 text-xs sm:text-right">{networkData.IPAM.Config[0].Subnet}</code>
                       </div>
                       {#if networkData.IPAM.Config[0].Gateway}
                         <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                           <span class="text-muted-foreground">{$messages.docker.networks.gateway}</span>
                           <code class="break-all rounded bg-muted px-1 py-0.5 text-xs sm:text-right">{networkData.IPAM.Config[0].Gateway}</code>
                         </div>
                       {/if}
                     {:else}
                       <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                         <span class="text-muted-foreground">{$messages.docker.networks.subnet}</span>
                         <span class="text-muted-foreground">-</span>
                       </div>
                     {/if}
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.ipv4Enabled}</span>
                      <Badge variant={networkData.EnableIPv4 === false ? 'secondary' : 'default'}>
                        {networkData.EnableIPv4 !== false ? $messages.docker.containers.yes : $messages.docker.containers.no}
                      </Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.ipv6Enabled}</span>
                      <Badge variant={networkData.EnableIPv6 ? 'default' : 'secondary'}>
                        {networkData.EnableIPv6 ? $messages.docker.containers.yes : $messages.docker.containers.no}
                      </Badge>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base" level="3">{$messages.docker.networks.accessControl}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.internal}</span>
                      <Badge variant={networkData.Internal ? 'outline' : 'secondary'}>
                        {networkData.Internal ? $messages.docker.containers.yes : $messages.docker.containers.no}
                      </Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.attachable}</span>
                      <Badge variant={networkData.Attachable ? 'default' : 'secondary'}>
                        {networkData.Attachable ? $messages.docker.containers.yes : $messages.docker.containers.no}
                      </Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.networks.ingress}</span>
                      <Badge variant={networkData.Ingress ? 'outline' : 'secondary'}>
                        {networkData.Ingress ? $messages.docker.containers.yes : $messages.docker.containers.no}
                      </Badge>
                    </div>
                  </CardContent>
                </Card>
              </div>

              {#if networkData.Labels && Object.keys(networkData.Labels).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base" level="3">{$messages.docker.networks.labels}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each Object.entries(networkData.Labels) as [key, value]}
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

            <TabsContent value="containers" class="space-y-4">
              {#if networkData.Containers && Object.keys(networkData.Containers).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base" level="3">{$messages.docker.networks.connectedContainers} ({Object.keys(networkData.Containers).length})</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      {#each Object.entries(networkData.Containers) as [endpointId, container]}
                        {@const typedContainer = container as { Name?: string; EndpointID?: string; IPv4Address?: string; IPv6Address?: string; MacAddress?: string }}
                        <div class="border-b border-border/50 last:border-0 pb-3 last:pb-0">
                          <div class="mb-2 flex flex-wrap items-center gap-2">
                            <span class="font-medium">{typedContainer.Name}</span>
                            <code class="text-xs bg-muted px-1 py-0.5 rounded">{endpointId?.substring(0, 12)}</code>
                          </div>
                          <div class="grid gap-1 text-sm">
                            {#if typedContainer.IPv4Address}
                              <div class="flex flex-col gap-1 sm:flex-row sm:gap-2">
                                <span class="text-muted-foreground w-12">{$messages.docker.networks.ipv4}:</span>
                                <code class="break-all rounded bg-muted px-1 py-0.5 text-xs">{typedContainer.IPv4Address}</code>
                              </div>
                            {/if}
                            {#if typedContainer.IPv6Address}
                              <div class="flex flex-col gap-1 sm:flex-row sm:gap-2">
                                <span class="text-muted-foreground w-12">{$messages.docker.networks.ipv6}:</span>
                                <code class="break-all rounded bg-muted px-1 py-0.5 text-xs">{typedContainer.IPv6Address}</code>
                              </div>
                            {/if}
                            {#if typedContainer.MacAddress}
                              <div class="flex flex-col gap-1 sm:flex-row sm:gap-2">
                                <span class="text-muted-foreground w-12">{$messages.docker.networks.mac}:</span>
                                <code class="break-all rounded bg-muted px-1 py-0.5 text-xs">{typedContainer.MacAddress}</code>
                              </div>
                            {/if}
                          </div>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {:else}
                <div class="text-sm text-muted-foreground">{$messages.docker.networks.noContainersConnected}</div>
              {/if}
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base" level="3">{$messages.docker.networks.rawJson}</CardTitle>
                  <CardDescription>{$messages.docker.networks.rawJsonDescription}</CardDescription>
                </CardHeader>
                <CardContent>
                  <pre class="code-surface max-h-[360px] overflow-auto break-all sm:max-h-[600px]">{JSON.stringify(networkData, null, 2)}</pre>
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
