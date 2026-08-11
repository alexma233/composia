<script lang="ts">
  import { Skeleton } from "$lib/components/ui/skeleton";

  interface Props {
    label: string;
  }

  let { label }: Props = $props();
</script>

<div class="page-shell" role="status" aria-live="polite" aria-busy="true">
  <span class="sr-only">{label}</span>
  <div class="page-stack">
    <Skeleton class="h-9 w-48" />
    <div class="grid gap-4 sm:grid-cols-2">
      {#each Array(2) as _}
        <div class="rounded-xl border bg-card p-6 space-y-4">
          <Skeleton class="h-4 w-28" />
          <Skeleton class="h-9 w-20" />
          <Skeleton class="h-3 w-40" />
        </div>
      {/each}
    </div>
    <div class="grid gap-4 lg:grid-cols-2">
      {#each Array(2) as _}
        <div class="rounded-xl border bg-card p-6 space-y-4">
          <Skeleton class="h-6 w-32" />
          {#each Array(4) as _}
            <Skeleton class="h-16 w-full" />
          {/each}
        </div>
      {/each}
    </div>
  </div>
</div>
