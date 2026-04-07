<script lang="ts">
  import type { PageData } from './$types';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge } from '$lib/components/ui/badge';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let networkData = $state<any>(null);
  let parseError = $state<string | null>(null);

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
      parseError = error instanceof Error ? error.message : 'Failed to parse network data';
      networkData = null;
    }
  });

  function formatDate(timestamp: string): string {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleString();
  }

  function getDriverColor(driver: string): 'default' | 'outline' | 'secondary' {
    const d = driver?.toLowerCase() || '';
    if (d === 'bridge') return 'default';
    if (d === 'host') return 'outline';
    if (d === 'overlay') return 'default';
    return 'secondary';
  }
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">
              {#if networkData}
                {networkData.Name || data.networkId.substring(0, 12)}
              {:else}
                Network
              {/if}
            </CardTitle>
            <CardDescription class="page-description">
              {#if networkData}
                <code class="text-xs bg-muted px-1 py-0.5 rounded">{networkData.Id?.substring(0, 19)}</code>
              {:else}
                {data.networkId}
              {/if}
            </CardDescription>
          </div>
          <div class="flex items-center gap-2">
            {#if networkData}
              <Badge variant={networkData.Scope === 'local' ? 'secondary' : 'outline'}>
                {networkData.Scope}
              </Badge>
            {/if}
            <a href="/nodes/{data.nodeId}/docker/networks" class="text-sm text-muted-foreground hover:underline">
              Back to networks
            </a>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        {#if data.error}
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            {data.error}
          </div>
        {:else if parseError}
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            Failed to parse network data: {parseError}
          </div>
        {:else if networkData}
          <Tabs value="info" class="w-full">
            <TabsList class="mb-4">
              <TabsTrigger value="info">Info</TabsTrigger>
              <TabsTrigger value="containers">Containers</TabsTrigger>
              <TabsTrigger value="raw">JSON</TabsTrigger>
            </TabsList>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-3">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Configuration</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Driver</span>
                      <Badge variant={getDriverColor(networkData.Driver)}>{networkData.Driver}</Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Scope</span>
                      <span>{networkData.Scope}</span>
                    </div>
                    {#if networkData.Labels && Object.keys(networkData.Labels).length > 0 && networkData.Labels['com.docker.compose.project']}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Compose Project</span>
                        <span>{networkData.Labels['com.docker.compose.project']}</span>
                      </div>
                    {/if}
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Created</span>
                      <span>{formatDate(networkData.Created)}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Network Settings</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    {#if networkData.IPAM?.Config && networkData.IPAM.Config.length > 0}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Subnet</span>
                        <code class="text-xs bg-muted px-1 py-0.5 rounded">{networkData.IPAM.Config[0].Subnet}</code>
                      </div>
                      {#if networkData.IPAM.Config[0].Gateway}
                        <div class="flex justify-between">
                          <span class="text-muted-foreground">Gateway</span>
                          <code class="text-xs bg-muted px-1 py-0.5 rounded">{networkData.IPAM.Config[0].Gateway}</code>
                        </div>
                      {/if}
                    {:else}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Subnet</span>
                        <span class="text-muted-foreground">-</span>
                      </div>
                    {/if}
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">IPv4 Enabled</span>
                      <Badge variant={networkData.EnableIPv4 === false ? 'secondary' : 'default'}>
                        {networkData.EnableIPv4 !== false ? 'Yes' : 'No'}
                      </Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">IPv6 Enabled</span>
                      <Badge variant={networkData.EnableIPv6 ? 'default' : 'secondary'}>
                        {networkData.EnableIPv6 ? 'Yes' : 'No'}
                      </Badge>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Access Control</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Internal</span>
                      <Badge variant={networkData.Internal ? 'outline' : 'secondary'}>
                        {networkData.Internal ? 'Yes' : 'No'}
                      </Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Attachable</span>
                      <Badge variant={networkData.Attachable ? 'default' : 'secondary'}>
                        {networkData.Attachable ? 'Yes' : 'No'}
                      </Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Ingress</span>
                      <Badge variant={networkData.Ingress ? 'outline' : 'secondary'}>
                        {networkData.Ingress ? 'Yes' : 'No'}
                      </Badge>
                    </div>
                  </CardContent>
                </Card>
              </div>

              {#if networkData.Labels && Object.keys(networkData.Labels).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Labels</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-1">
                      {#each Object.entries(networkData.Labels) as [key, value]}
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

            <TabsContent value="containers" class="space-y-4">
              {#if networkData.Containers && Object.keys(networkData.Containers).length > 0}
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Connected Containers ({Object.keys(networkData.Containers).length})</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      {#each Object.entries(networkData.Containers) as [endpointId, container]}
                        {@const typedContainer = container as { Name?: string; EndpointID?: string; IPv4Address?: string; IPv6Address?: string; MacAddress?: string }}
                        <div class="border-b border-border/50 last:border-0 pb-3 last:pb-0">
                          <div class="flex items-center gap-2 mb-2">
                            <span class="font-medium">{typedContainer.Name}</span>
                            <code class="text-xs bg-muted px-1 py-0.5 rounded">{endpointId?.substring(0, 12)}</code>
                          </div>
                          <div class="grid gap-1 text-sm">
                            {#if typedContainer.IPv4Address}
                              <div class="flex gap-2">
                                <span class="text-muted-foreground w-12">IPv4:</span>
                                <code class="text-xs bg-muted px-1 py-0.5 rounded">{typedContainer.IPv4Address}</code>
                              </div>
                            {/if}
                            {#if typedContainer.IPv6Address}
                              <div class="flex gap-2">
                                <span class="text-muted-foreground w-12">IPv6:</span>
                                <code class="text-xs bg-muted px-1 py-0.5 rounded">{typedContainer.IPv6Address}</code>
                              </div>
                            {/if}
                            {#if typedContainer.MacAddress}
                              <div class="flex gap-2">
                                <span class="text-muted-foreground w-12">MAC:</span>
                                <code class="text-xs bg-muted px-1 py-0.5 rounded">{typedContainer.MacAddress}</code>
                              </div>
                            {/if}
                          </div>
                        </div>
                      {/each}
                    </div>
                  </CardContent>
                </Card>
              {:else}
                <div class="text-sm text-muted-foreground">No containers connected to this network</div>
              {/if}
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Raw JSON</CardTitle>
                  <CardDescription>Full network inspection data in JSON format</CardDescription>
                </CardHeader>
                <CardContent>
                  <pre class="text-xs font-mono overflow-auto whitespace-pre-wrap break-all bg-background/80 p-4 rounded-lg border border-border/70 max-h-[600px]">{JSON.stringify(networkData, null, 2)}</pre>
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
