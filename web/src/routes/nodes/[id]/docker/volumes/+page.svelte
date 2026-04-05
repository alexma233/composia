<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatTimestamp } from '$lib/presenters';

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
            <CardTitle class="page-title">Volumes</CardTitle>
            <CardDescription class="page-description">
              Docker volumes on {data.nodeId}
            </CardDescription>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground hover:underline">
            ← Back to node
          </a>
        </div>
      </CardHeader>
      <CardContent>
        {#if data.volumes && data.volumes.length > 0}
          <div class="space-y-3">
            {#each data.volumes as volume}
              <div class="rounded-lg border border-border/70 bg-background/80 p-4">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="space-y-1">
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-sm font-medium">{volume.name}</span>
                      <button
                        on:click={() => copyToClipboard(volume.name)}
                        class="text-xs text-muted-foreground hover:text-foreground"
                        title="Copy name"
                      >
                        📋
                      </button>
                    </div>
                    <div class="text-xs text-muted-foreground">
                      Driver: {volume.driver} · Scope: {volume.scope}
                    </div>
                    {#if volume.created}
                      <div class="text-xs text-muted-foreground">
                        Created: {volume.created}
                      </div>
                    {/if}
                  </div>
                  <a
                    href="/nodes/{data.nodeId}/docker/volumes/{encodeURIComponent(volume.name)}"
                    class="text-sm text-muted-foreground hover:text-foreground hover:underline"
                  >
                    Inspect →
                  </a>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state">No volumes found.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
