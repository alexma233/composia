<script lang="ts">
  import { invalidateAll } from "$app/navigation";
  import type { PageData, ActionData } from "./$types";
  import { enhance } from "$app/forms";
  import { onMount } from "svelte";

  import {
    capabilityReasonMessage,
    nodeActionCapability,
  } from "$lib/capabilities";
  import DisabledReasonTooltip from "$lib/components/app/disabled-reason-tooltip.svelte";
  import {
    Alert,
    AlertDescription,
    AlertTitle,
  } from "$lib/components/ui/alert";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogOverlay,
    DialogTitle,
  } from "$lib/components/ui/dialog";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
  } from "$lib/components/ui/dropdown-menu";
  import { ChevronDown } from "lucide-svelte";
  import { startPolling } from "$lib/refresh";
  import {
    formatBytes,
    formatTimestamp,
    onlineStatusTone,
    taskStatusLabel,
    taskStatusTone,
  } from "$lib/presenters";
  import { messages } from "$lib/i18n";

  interface Props {
    data: PageData;
    form: ActionData;
  }

  type PruneTarget =
    | "containers"
    | "images"
    | "images_all"
    | "networks"
    | "volumes"
    | "all"
    | "system_all"
    | "system_all_volumes";

  let { data, form }: Props = $props();
  let imagePruneTarget = $state<"images_all" | "images">("images_all");
  let systemPruneTarget = $state<"system_all_volumes" | "system_all" | "all">(
    "system_all_volumes",
  );
  let pruneForm = $state<HTMLFormElement | null>(null);
  let pruneTargetInput = $state<PruneTarget>("images_all");
  let pruneDialogOpen = $state(false);
  let pendingPruneTarget = $state<PruneTarget | null>(null);
  let caddySyncCapability = $derived(
    nodeActionCapability(data.node?.actions, "caddySync"),
  );
  let caddyReloadCapability = $derived(
    nodeActionCapability(data.node?.actions, "caddyReload"),
  );
  let caddySyncReason = $derived(
    caddySyncCapability.enabled
      ? ""
      : capabilityReasonMessage(caddySyncCapability.reasonCode, $messages),
  );
  let caddyReloadReason = $derived(
    caddyReloadCapability.enabled
      ? ""
      : capabilityReasonMessage(caddyReloadCapability.reasonCode, $messages),
  );
  let formErrorMessage = $derived(
    form?.errorCode
      ? capabilityReasonMessage(form.errorCode, $messages)
      : (form?.error ?? ""),
  );

  onMount(() => startPolling(() => invalidateAll(), { intervalMs: 5000 }));

  function pruneLabel(target: PruneTarget) {
    switch (target) {
      case "containers":
        return $messages.nodes.docker.prune.containers;
      case "images":
        return $messages.nodes.docker.prune.images;
      case "images_all":
        return $messages.nodes.docker.prune.imagesAll;
      case "networks":
        return $messages.nodes.docker.prune.networks;
      case "volumes":
        return $messages.nodes.docker.prune.volumes;
      case "system_all":
        return $messages.nodes.docker.prune.systemAll;
      case "system_all_volumes":
        return $messages.nodes.docker.prune.systemAllVolumes;
      case "all":
      default:
        return $messages.nodes.docker.prune.all;
    }
  }

  function requiresPruneWarning(target: PruneTarget) {
    return (
      target === "containers" ||
      target === "volumes" ||
      target === "all" ||
      target === "system_all" ||
      target === "system_all_volumes"
    );
  }

  function submitPrune(target: PruneTarget) {
    pruneTargetInput = target;
    pruneForm?.requestSubmit();
  }

  function requestPrune(target: PruneTarget) {
    if (requiresPruneWarning(target)) {
      pendingPruneTarget = target;
      pruneDialogOpen = true;
      return;
    }
    submitPrune(target);
  }

  function confirmPrune() {
    if (!pendingPruneTarget) {
      return;
    }
    pruneDialogOpen = false;
    submitPrune(pendingPruneTarget);
    pendingPruneTarget = null;
  }

  let pruneWarningDescription = $derived.by(() => {
    const target = pendingPruneTarget;
    const nodeId = data.node?.nodeId ?? data.node?.displayName ?? "";
    if (!target) {
      return "";
    }
    switch (target) {
      case "containers":
        return $messages.nodes.docker.prune.containersWarning.replace(
          "{nodeId}",
          nodeId,
        );
      case "volumes":
        return $messages.nodes.docker.prune.volumesWarning.replace(
          "{nodeId}",
          nodeId,
        );
      case "system_all":
        return $messages.nodes.docker.prune.systemAllWarning.replace(
          "{nodeId}",
          nodeId,
        );
      case "system_all_volumes":
        return $messages.nodes.docker.prune.systemAllVolumesWarning.replace(
          "{nodeId}",
          nodeId,
        );
      case "all":
      default:
        return $messages.nodes.docker.prune.systemWarning.replace(
          "{nodeId}",
          nodeId,
        );
    }
  });
</script>

<svelte:head>
  <title>{data.node?.displayName ?? $messages.nodes.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <Card>
      <CardHeader>
        {#if data.node}
          <div class="page-header">
            <div class="page-heading">
              <CardTitle class="page-title">{data.node.displayName}</CardTitle>
              {#if data.node.displayName !== data.node.nodeId}
                <p class="page-meta">
                  {data.node.nodeId} · {$messages.dashboard.lastHeartbeat}
                  {formatTimestamp(data.node.lastHeartbeat)}
                </p>
              {:else}
                <p class="page-meta">
                  {$messages.dashboard.lastHeartbeat}
                  {formatTimestamp(data.node.lastHeartbeat)}
                </p>
              {/if}
            </div>
            <Badge variant={onlineStatusTone(data.node.isOnline)}>
              {data.node.isOnline
                ? $messages.status.online
                : $messages.status.offline}
            </Badge>
          </div>
        {/if}

        {#if data.error}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
            <AlertDescription>{data.error}</AlertDescription>
          </Alert>
        {/if}
      </CardHeader>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle class="section-title"
          >{$messages.nodes.docker.title}</CardTitle
        >
      </CardHeader>
      <CardContent>
        {#if data.dockerStats}
          <div class="space-y-4">
            <div class="summary-grid sm:grid-cols-4 xl:grid-cols-4">
              <a
                href="/nodes/{data.node?.nodeId}/docker/containers"
                class="stat-link"
              >
                <div class="text-2xl font-semibold">
                  {data.dockerStats.containersRunning}/{data.dockerStats
                    .containersTotal}
                </div>
                <div class="text-xs text-muted-foreground">
                  {$messages.nodes.docker.containers}
                </div>
              </a>
              <a
                href="/nodes/{data.node?.nodeId}/docker/images"
                class="stat-link"
              >
                <div class="text-2xl font-semibold">
                  {data.dockerStats.images}
                </div>
                <div class="text-xs text-muted-foreground">
                  {$messages.nodes.docker.images}
                </div>
              </a>
              <a
                href="/nodes/{data.node?.nodeId}/docker/networks"
                class="stat-link"
              >
                <div class="text-2xl font-semibold">
                  {data.dockerStats.networks}
                </div>
                <div class="text-xs text-muted-foreground">
                  {$messages.nodes.docker.networks}
                </div>
              </a>
              <a
                href="/nodes/{data.node?.nodeId}/docker/volumes"
                class="stat-link"
              >
                <div class="text-2xl font-semibold">
                  {data.dockerStats.volumes}
                </div>
                <div class="text-xs text-muted-foreground">
                  {$messages.nodes.docker.volumes}
                </div>
              </a>
            </div>

            <div class="text-sm text-muted-foreground">
              {$messages.nodes.docker.version}
              {data.dockerStats.dockerServerVersion || $messages.common.unknown}
              {#if data.dockerStats.volumesSizeBytes > 0}
                · {formatBytes(data.dockerStats.volumesSizeBytes)}
                {$messages.nodes.docker.volumesSize}
              {/if}
              {#if data.dockerStats.disksUsageBytes > 0}
                · {formatBytes(data.dockerStats.disksUsageBytes)}
                {$messages.nodes.docker.diskUsage}
              {/if}
            </div>

            {#if formErrorMessage}
              <Alert variant="destructive">
                <AlertDescription>{formErrorMessage}</AlertDescription>
              </Alert>
            {/if}

            <form
              bind:this={pruneForm}
              method="POST"
              action="?/prune"
              use:enhance
              class="hidden"
            >
              <input
                type="hidden"
                name="target"
                bind:value={pruneTargetInput}
              />
            </form>

            {#if data.node?.isOnline}
              <div class="flex flex-wrap gap-2">
                <form method="POST" action="?/syncCaddyFiles" use:enhance>
                  <DisabledReasonTooltip reason={caddySyncReason}>
                    <Button
                      variant="outline"
                      size="sm"
                      type="submit"
                      disabled={!caddySyncCapability.enabled}
                      >{$messages.nodes.docker.rebuildCaddy}</Button
                    >
                  </DisabledReasonTooltip>
                </form>
                <form method="POST" action="?/reloadCaddy" use:enhance>
                  <DisabledReasonTooltip reason={caddyReloadReason}>
                    <Button
                      variant="outline"
                      size="sm"
                      type="submit"
                      disabled={!caddyReloadCapability.enabled}
                      >{$messages.nodes.docker.reloadCaddy}</Button
                    >
                  </DisabledReasonTooltip>
                </form>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onclick={() => requestPrune("containers")}
                >
                  {$messages.nodes.docker.prune.containers}
                </Button>
                <div class="flex items-center">
                  <Button
                    variant="outline"
                    size="sm"
                    type="button"
                    class="rounded-r-none border-r-0"
                    onclick={() => requestPrune(imagePruneTarget)}
                  >
                    {imagePruneTarget === "images_all"
                      ? $messages.nodes.docker.prune.imagesAll
                      : $messages.nodes.docker.prune.images}
                  </Button>
                  <DropdownMenu>
                    <DropdownMenuTrigger>
                      <Button
                        variant="outline"
                        size="sm"
                        type="button"
                        class="rounded-l-none px-2"
                      >
                        <ChevronDown class="size-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                      <DropdownMenuItem
                        onclick={() => (imagePruneTarget = "images_all")}
                      >
                        {$messages.nodes.docker.prune.imagesAll}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onclick={() => (imagePruneTarget = "images")}
                      >
                        {$messages.nodes.docker.prune.images}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onclick={() => requestPrune("networks")}
                >
                  {$messages.nodes.docker.prune.networks}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onclick={() => requestPrune("volumes")}
                >
                  {$messages.nodes.docker.prune.volumes}
                </Button>
                <div class="flex items-center">
                  <Button
                    variant="outline"
                    size="sm"
                    type="button"
                    class="rounded-r-none border-r-0"
                    onclick={() => requestPrune(systemPruneTarget)}
                  >
                    {systemPruneTarget === "system_all_volumes"
                      ? $messages.nodes.docker.prune.systemAllVolumes
                      : systemPruneTarget === "system_all"
                        ? $messages.nodes.docker.prune.systemAll
                        : $messages.nodes.docker.prune.all}
                  </Button>
                  <DropdownMenu>
                    <DropdownMenuTrigger>
                      <Button
                        variant="outline"
                        size="sm"
                        type="button"
                        class="rounded-l-none px-2"
                      >
                        <ChevronDown class="size-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                      <DropdownMenuItem
                        onclick={() =>
                          (systemPruneTarget = "system_all_volumes")}
                      >
                        {$messages.nodes.docker.prune.systemAllVolumes}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onclick={() => (systemPruneTarget = "system_all")}
                      >
                        {$messages.nodes.docker.prune.systemAll}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onclick={() => (systemPruneTarget = "all")}
                      >
                        {$messages.nodes.docker.prune.all}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            {:else}
              <div class="text-sm text-muted-foreground">
                {$messages.nodes.docker.nodeOffline}
              </div>
            {/if}
          </div>
        {:else}
          <div class="text-sm text-muted-foreground">
            {$messages.nodes.docker.noStats}
          </div>
        {/if}
      </CardContent>
    </Card>

    <Dialog bind:open={pruneDialogOpen}>
      <DialogOverlay />
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{$messages.nodes.docker.prune.dialogTitle}</DialogTitle>
          <DialogDescription>{pruneWarningDescription}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onclick={() => {
              pruneDialogOpen = false;
              pendingPruneTarget = null;
            }}
          >
            {$messages.common.cancel}
          </Button>
          <Button
            type="button"
            variant="destructive"
            onclick={() => confirmPrune()}
            disabled={!pendingPruneTarget}
          >
            {$messages.nodes.docker.prune.confirmAction}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Card>
      <CardHeader class="section-header">
        <div class="section-heading">
          <CardTitle class="section-title">
            {#if data.node}
              <a
                class="hover:text-foreground/80 transition-colors"
                href={`/tasks?nodeId=${encodeURIComponent(data.node.nodeId)}`}
              >
                {$messages.dashboard.recentTasks}
              </a>
            {:else}
              {$messages.dashboard.recentTasks}
            {/if}
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.tasks as task}
            <a href={`/tasks/${task.taskId}`} class="list-row">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-medium">{task.type}</div>
                  <div class="truncate text-xs text-muted-foreground">
                    {task.serviceName ?? $messages.tasks.nodeLevel}
                  </div>
                </div>
                <Badge variant={taskStatusTone(task.status)}
                  >{taskStatusLabel(task.status, $messages)}</Badge
                >
              </div>
              <div class="mt-2 text-xs text-muted-foreground">
                {formatTimestamp(task.createdAt)}
              </div>
            </a>
          {/each}
          {#if !data.tasks.length}
            <div class="empty-state">{$messages.tasks.noTasks}</div>
          {/if}
        </div>
      </CardContent>
    </Card>
  </div>
</div>
