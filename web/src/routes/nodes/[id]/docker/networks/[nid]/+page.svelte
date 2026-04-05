<script lang="ts">
  import type { PageData } from './$types';

  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  export let data: PageData;
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">Network: {data.networkId}</CardTitle>
            <CardDescription class="page-description">
              Docker network details
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}/docker/networks" class="text-sm text-muted-foreground hover:underline">
            ← Back to networks
          </a>
        </div>
      </CardHeader>
      <CardContent>
        {#if data.rawJson}
          <pre class="text-xs font-mono overflow-auto whitespace-pre-wrap break-all bg-background/80 p-4 rounded-lg border border-border/70">{data.rawJson}</pre>
        {:else if data.error}
          <div class="text-sm text-muted-foreground">Error loading network: {data.error}</div>
        {:else}
          <div class="text-sm text-muted-foreground">Loading...</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
