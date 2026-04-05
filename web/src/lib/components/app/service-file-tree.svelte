<script lang="ts">
  import { ChevronDown, ChevronRight, FileText, Folder } from 'lucide-svelte';

  import type { ServiceFileNode } from '$lib/service-workspace';

  export let nodes: ServiceFileNode[] = [];
  export let activePath = '';
  export let collapsedPaths: Set<string> = new Set();
  export let depth = 0;
  export let onSelect: (path: string) => void = () => {};
  export let onToggle: (path: string) => void = () => {};
</script>

<div class="space-y-1">
  {#each nodes as node}
    <div>
      {#if node.isDir}
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          style={`padding-left:${depth * 12 + 8}px`}
          on:click={() => onToggle(node.path)}
        >
          {#if collapsedPaths.has(node.path)}
            <ChevronRight class="size-4" />
          {:else}
            <ChevronDown class="size-4" />
          {/if}
          <Folder class="size-4" />
          <span class="truncate">{node.name}</span>
        </button>

        {#if !collapsedPaths.has(node.path)}
          <svelte:self
            nodes={node.children}
            activePath={activePath}
            collapsedPaths={collapsedPaths}
            depth={depth + 1}
            {onSelect}
            {onToggle}
          />
        {/if}
      {:else}
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors"
          class:bg-secondary={activePath === node.path}
          class:text-secondary-foreground={activePath === node.path}
          class:text-muted-foreground={activePath !== node.path}
          style={`padding-left:${depth * 12 + 28}px`}
          on:click={() => onSelect(node.path)}
        >
          <FileText class="size-4" />
          <span class="truncate">{node.name}</span>
        </button>
      {/if}
    </div>
  {/each}
</div>
