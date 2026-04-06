<script lang="ts">
  import { ChevronDown, ChevronRight, FileText, Folder } from 'lucide-svelte';

  import type { ServiceFileNode } from '$lib/service-workspace';
  import ServiceFileTree from './service-file-tree.svelte';

  interface Props {
    nodes?: ServiceFileNode[];
    activePath?: string;
    selectedPath?: string;
    collapsedPaths?: Set<string>;
    depth?: number;
    onOpenFile?: (path: string) => void;
    onSelectNode?: (path: string) => void;
    onToggle?: (path: string) => void;
  }

  let {
    nodes = [],
    activePath = '',
    selectedPath = '',
    collapsedPaths = new Set(),
    depth = 0,
    onOpenFile = () => {},
    onSelectNode = () => {},
    onToggle = () => {}
  }: Props = $props();
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
          style="padding-left:{depth * 12 + 8}px"
          onclick={() => {
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
          <ServiceFileTree
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
          style="padding-left:{depth * 12 + 28}px"
          onclick={() => {
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
