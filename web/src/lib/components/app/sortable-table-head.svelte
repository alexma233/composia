<script lang="ts">
  import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-svelte';
  import { TableHead } from '$lib/components/ui/table';

  let {
    field,
    label,
    sortField,
    sortDirection,
    onSort,
    class: className,
    ...restProps
  }: {
    field: string;
    label: string;
    sortField: string;
    sortDirection: 'asc' | 'desc';
    onSort: (field: string) => void;
    class?: string;
    [key: string]: unknown;
  } = $props();

  let sorted = $derived(sortField === field);
</script>

<TableHead class={className} {...restProps}>
  <button
    type="button"
    class="flex items-center gap-1 hover:text-foreground"
    onclick={() => onSort(field)}
  >
    {label}
    {#if !sorted}
      <ChevronsUpDown class="h-3 w-3 text-muted-foreground" />
    {:else if sortDirection === 'asc'}
      <ChevronUp class="h-3 w-3" />
    {:else}
      <ChevronDown class="h-3 w-3" />
    {/if}
  </button>
</TableHead>
