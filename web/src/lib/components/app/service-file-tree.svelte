<script lang="ts">
  import { ChevronDown, ChevronRight, FileText, Folder } from "lucide-svelte";

  import type { ServiceFileNode } from "$lib/service-workspace";
  import ServiceFileTree from "./service-file-tree.svelte";

  interface Props {
    nodes?: ServiceFileNode[];
    activePath?: string;
    selectedPath?: string;
    collapsedPaths?: Set<string>;
    depth?: number;
    iconTheme?: "light" | "dark";
    onOpenFile?: (path: string) => void;
    onSelectNode?: (path: string) => void;
    onToggle?: (path: string) => void;
  }

  let {
    nodes = [],
    activePath = "",
    selectedPath = "",
    collapsedPaths = new Set(),
    depth = 0,
    iconTheme = "dark",
    onOpenFile = () => {},
    onSelectNode = () => {},
    onToggle = () => {},
  }: Props = $props();

  function materialIconUrl(iconName: string) {
    return `/material-icons/${encodeURIComponent(iconName)}`;
  }

  function nodeIconName(node: ServiceFileNode, expanded = false) {
    if (iconTheme === "light") {
      return expanded
        ? (node.expandedLightIconName ??
            node.lightIconName ??
            node.expandedIconName ??
            node.iconName ??
            null)
        : (node.lightIconName ?? node.iconName ?? null);
    }

    return expanded
      ? (node.expandedIconName ?? node.iconName ?? null)
      : (node.iconName ?? null);
  }
</script>

<div class="space-y-1">
  {#each nodes as node}
    <div>
      {#if node.isDir}
        {@const expanded = !collapsedPaths.has(node.path)}
        {@const iconName = nodeIconName(node, expanded)}
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
          {#if iconName}
            <img
              src={materialIconUrl(iconName)}
              alt=""
              aria-hidden="true"
              class="size-4 shrink-0"
              decoding="async"
            />
          {:else}
            <Folder class="size-4" />
          {/if}
          <span class="truncate">{node.name}</span>
        </button>

        {#if expanded}
          <ServiceFileTree
            nodes={node.children}
            {activePath}
            {selectedPath}
            {collapsedPaths}
            depth={depth + 1}
            {iconTheme}
            {onOpenFile}
            {onSelectNode}
            {onToggle}
          />
        {/if}
      {:else}
        {@const iconName = nodeIconName(node)}
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
          {#if iconName}
            <img
              src={materialIconUrl(iconName)}
              alt=""
              aria-hidden="true"
              class="size-4 shrink-0"
              decoding="async"
            />
          {:else}
            <FileText class="size-4" />
          {/if}
          <span class="truncate">{node.name}</span>
        </button>
      {/if}
    </div>
  {/each}
</div>
