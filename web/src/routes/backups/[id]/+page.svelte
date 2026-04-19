<script lang="ts">
  import { goto } from "$app/navigation";
  import { toast } from "svelte-sonner";

  import {
    actionErrorMessage,
    backupActionCapability,
    capabilityReasonMessage,
  } from "$lib/capabilities";
  import type { PageData } from "./$types";
  import type { NodeSummary } from "$lib/server/controller";
  import { messages } from "$lib/i18n";

  import {
    Alert,
    AlertDescription,
    AlertTitle,
  } from "$lib/components/ui/alert";
  import DisabledReasonTooltip from "$lib/components/app/disabled-reason-tooltip.svelte";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import { Label } from "$lib/components/ui/label";
  import * as Select from "$lib/components/ui/select";
  import {
    formatTimestamp,
    taskStatusLabel,
    taskStatusTone,
  } from "$lib/presenters";

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let targetNodeId = $state("");
  let restoring = $state(false);
  let eligibleRestoreNodes = $derived(
    data.nodes.filter((node: NodeSummary) => node.enabled && node.isOnline),
  );
  let restoreCapability = $derived(
    backupActionCapability(data.backup?.actions, "restore"),
  );
  let restoreReason = $derived(
    restoreCapability.enabled
      ? ""
      : capabilityReasonMessage(restoreCapability.reasonCode, $messages),
  );

  $effect(() => {
    targetNodeId = eligibleRestoreNodes[0]?.nodeId ?? "";
  });

  async function startRestore() {
    if (
      !data.backup?.backupId ||
      !targetNodeId ||
      restoring ||
      !restoreCapability.enabled
    ) {
      return;
    }

    restoring = true;

    try {
      const response = await fetch(`/backups/${data.backup.backupId}/restore`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nodeId: targetNodeId }),
      });
      const payload = (await response.json()) as {
        taskId?: string;
        error?: string;
        reasonCode?: string;
      };
      if (!response.ok || !payload.taskId) {
        throw new Error(
          actionErrorMessage(
            payload,
            $messages,
            $messages.backups.restoreFailed,
          ),
        );
      }

      toast.success(
        $messages.backups.restoreQueued.replace("{taskId}", payload.taskId),
      );
      goto(`/tasks/${payload.taskId}`);
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : $messages.backups.restoreFailed,
      );
    } finally {
      restoring = false;
    }
  }
</script>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title"
            >{$messages.backups.detailsTitle}</CardTitle
          >
          {#if data.backup}
            <div class="page-meta">
              {data.backup.serviceName} / {data.backup.dataName}
            </div>
          {/if}
        </div>
        {#if data.backup}
          <Badge variant={taskStatusTone(data.backup.status)}>
            {taskStatusLabel(data.backup.status, $messages)}
          </Badge>
        {/if}
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    {#if data.backup}
      <CardContent class="space-y-4">
        <div class="summary-grid">
          <div class="metric-card">
            <div class="metric-label">{$messages.backups.backupId}</div>
            <div class="mt-2 break-all text-sm text-foreground">
              {data.backup.backupId}
            </div>
          </div>
          <div class="metric-card">
            <div class="metric-label">{$messages.backups.sourceTask}</div>
            <div class="mt-2 text-sm text-foreground">
              <a
                href={`/tasks/${data.backup.taskId}`}
                class="hover:text-primary">{data.backup.taskId}</a
              >
            </div>
          </div>
          <div class="metric-card">
            <div class="metric-label">{$messages.common.started}</div>
            <div class="mt-2 text-sm text-foreground">
              {formatTimestamp(data.backup.startedAt)}
            </div>
          </div>
          <div class="metric-card">
            <div class="metric-label">{$messages.common.finished}</div>
            <div class="mt-2 text-sm text-foreground">
              {formatTimestamp(data.backup.finishedAt)}
            </div>
          </div>
        </div>

        <div class="inset-card">
          <div class="metric-label">{$messages.backups.artifactRef}</div>
          <div class="mt-2 break-all text-sm text-foreground">
            {data.backup.artifactRef || $messages.backups.noArtifact}
          </div>
        </div>

        {#if data.backup.errorSummary}
          <Alert variant="destructive">
            <AlertTitle>{$messages.common.error}</AlertTitle>
            <AlertDescription>{data.backup.errorSummary}</AlertDescription>
          </Alert>
        {/if}

        <div class="inset-card space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="metric-label">{$messages.backups.restore}</div>
              <div class="mt-1 text-sm text-muted-foreground">
                {$messages.backups.targetNode}
              </div>
            </div>
            <DisabledReasonTooltip reason={restoreReason}>
              <Button
                type="button"
                onclick={startRestore}
                disabled={restoring ||
                  !targetNodeId ||
                  data.backup.status !== "succeeded" ||
                  !data.backup.artifactRef ||
                  !restoreCapability.enabled}
              >
                {restoring
                  ? $messages.backups.restoring
                  : $messages.backups.restore}
              </Button>
            </DisabledReasonTooltip>
          </div>

          <div class="space-y-2">
            <Label for="restore-target-node"
              >{$messages.backups.selectTargetNode}</Label
            >
            <Select.Root type="single" bind:value={targetNodeId as any}>
              <Select.Trigger id="restore-target-node" class="w-full">
                {targetNodeId || $messages.backups.selectTargetNode}
              </Select.Trigger>
              <Select.Content>
                {#each eligibleRestoreNodes as node}
                  <Select.Item value={node.nodeId} label={node.displayName}>
                    {node.displayName} ({node.nodeId})
                  </Select.Item>
                {/each}
              </Select.Content>
            </Select.Root>
          </div>
        </div>
      </CardContent>
    {/if}
  </Card>
</div>
