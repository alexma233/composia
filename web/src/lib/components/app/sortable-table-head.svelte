<script lang="ts">
  import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-svelte';
  import { TableHead } from '$lib/components/ui/table';

  interface Props {
    field: string;
    label: string;
    sortField: string;
    sortDirection: 'asc' | 'desc';
    onSort: (field: string) => void;
    class?: string;
    [key: string]: unknown;
  }

  let {
    field,
    label,
    sortField,
    sortDirection,
    onSort,
    class: className,
    ...restProps
  }: Props = $props();

  let sorted = $derived(sortField === field);

  let ariaSort: 'ascending' | 'descending' | 'none' = $derived(
    sorted ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'
  );
</script>

<TableHead class={className} aria-sort={ariaSort} {...restProps}>
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
