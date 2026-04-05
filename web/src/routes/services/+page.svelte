<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { formatTimestamp, runtimeStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="rounded-lg border bg-card p-6 shadow-xs">
    <div class="mb-6 flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Services</h1>
        <p class="text-sm text-muted-foreground">Declared services and runtime state from the controller.</p>
      </div>
      <span class="rounded-md border bg-muted/50 px-2.5 py-1 text-xs text-muted-foreground">
        {data.services.length} loaded
      </span>
    </div>

    {#if data.error}
      <div class="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
        {data.error}
      </div>
    {/if}

    <div class="space-y-3">
      {#each data.services as service}
        <a href={`/services/${service.name}`} class="block rounded-lg border bg-background px-4 py-4 transition-colors hover:bg-muted/40">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-base font-medium">{service.name}</div>
              <div class="text-sm text-muted-foreground">Updated {formatTimestamp(service.updatedAt)}</div>
            </div>
            <Badge variant={runtimeStatusTone(service.runtimeStatus)}>
              {service.runtimeStatus}
            </Badge>
          </div>
        </a>
      {/each}

      {#if !data.services.length}
        <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
          No services loaded.
        </div>
      {/if}
    </div>
  </div>
</div>
