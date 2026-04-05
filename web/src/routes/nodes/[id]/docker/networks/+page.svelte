<script lang="ts">
  import type { PageData } from './$types';

  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';

  export let data: PageData;

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">Networks</CardTitle>
            <CardDescription class="page-description">
              Docker networks on {data.nodeId}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            ← Back to node
          </a>
        </div>
      </CardHeader>
      <CardContent>
        {#if data.networks && data.networks.length > 0}
          <div class="space-y-3">
            {#each data.networks as network}
              <div class="rounded-lg border border-border/70 bg-background/80 p-4">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="space-y-1">
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-sm font-medium">{network.name}</span>
                      <button
                        on:click={() => copyToClipboard(network.id)}
                        class="text-xs text-muted-foreground hover:text-foreground"
                        title="Copy ID"
                      >
                        📋
                      </button>
                    </div>
                    <div class="text-xs text-muted-foreground">
                      Driver: {network.driver} · Scope: {network.scope}
                      {#if network.internal}· Internal{/if}
                      {#if network.attachable}· Attachable{/if}
                    </div>
                    {#if network.created}
                      <div class="text-xs text-muted-foreground">
                        Created: {network.created}
                      </div>
                    {/if}
                  </div>
                  <a
                    href="/nodes/{data.nodeId}/docker/networks/{encodeURIComponent(network.id)}"
                    class="text-sm text-muted-foreground hover:text-foreground hover:underline"
                  >
                    Inspect →
                  </a>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state">No networks found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
