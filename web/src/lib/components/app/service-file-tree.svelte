<script lang="ts">
  import { ChevronDown, ChevronRight, FileText, Folder } from 'lucide-svelte';

  import type { ServiceFileNode } from '$lib/service-workspace';

export let nodes: ServiceFileNode[] = [];
export let activePath = '';
export let selectedPath = '';
export let collapsedPaths: Set<string> = new Set();
export let depth = 0;
export let onOpenFile: (path: string) => void = () => {};
export let onSelectNode: (path: string) => void = () => {};
export let onToggle: (path: string) => void = () => {};
</script>

<div class="space-y-1">
  {#each nodes as node}
    <div>
      {#if node.isDir}
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent/60"
          class:bg-secondary={selectedPath === node.path}
          class:text-secondary-foreground={selectedPath === node.path}
          class:text-muted-foreground={selectedPath !== node.path}
          style={`padding-left:${depth * 12 + 8}px`}
          on:click={() => {
            onSelectNode(node.path);
            onToggle(node.path);
          }}
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
            selectedPath={selectedPath}
            collapsedPaths={collapsedPaths}
            depth={depth + 1}
            {onOpenFile}
            {onSelectNode}
            {onToggle}
          />
        {/if}
      {:else}
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent/60"
          class:bg-secondary={activePath === node.path}
          class:text-secondary-foreground={activePath === node.path}
          class:text-muted-foreground={activePath !== node.path}
          style={`padding-left:${depth * 12 + 28}px`}
          on:click={() => {
            onSelectNode(node.path);
            onOpenFile(node.path);
          }}
        >
          <FileText class="size-4" />
          <span class="truncate">{node.name}</span>
        </button>
      {/if}
    </div>
  {/each}
</div>
