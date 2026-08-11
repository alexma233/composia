<script lang="ts">
  import { Skeleton } from "$lib/components/ui/skeleton";

  interface Props {
    label: string;
    columns?: number;
  }

  let { label, columns = 4 }: Props = $props();
</script>

<div class="page-shell" role="status" aria-live="polite" aria-busy="true">
  <span class="sr-only">{label}</span>
  <div class="rounded-xl border bg-card p-6">
    <div class="mb-8 flex items-center justify-between gap-4">
      <div class="space-y-3">
        <Skeleton class="h-8 w-48" />
        <Skeleton class="h-4 w-72 max-w-full" />
      </div>
      <Skeleton class="h-8 w-20" />
    </div>
    <div class="space-y-3">
      {#each Array(6) as _, row}
        <div
          class="grid min-h-12 items-center gap-4 border-b py-3"
          style={`grid-template-columns: repeat(${columns}, minmax(0, 1fr))`}
        >
          {#each Array(columns) as _, column}
            <Skeleton class={column === 0 ? "h-4 w-4/5" : "h-4 w-3/5"} />
          {/each}
        </div>
      {/each}
    </div>
  </div>
</div>
