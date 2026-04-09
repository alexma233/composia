<script lang="ts">
  import type { PageData, ActionData } from './$types';
  import { enhance } from '$app/forms';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatBytes, formatTimestamp, onlineStatusTone, taskStatusLabel, taskStatusTone } from '$lib/presenters';
  import { messages } from '$lib/i18n';

  interface Props {
    data: PageData;
    form: ActionData;
  }

  let { data, form }: Props = $props();
</script>

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
                  {data.node.nodeId} · {$messages.dashboard.lastHeartbeat} {formatTimestamp(data.node.lastHeartbeat)}
                </p>
              {:else}
                <p class="page-meta">
                  {$messages.dashboard.lastHeartbeat} {formatTimestamp(data.node.lastHeartbeat)}
                </p>
              {/if}
            </div>
            <Badge variant={onlineStatusTone(data.node.isOnline)}>
              {data.node.isOnline ? $messages.status.online : $messages.status.offline}
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
        <CardTitle class="section-title">{$messages.nodes.docker.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {#if data.dockerStats}
          <div class="space-y-4">
            <div class="summary-grid sm:grid-cols-4 xl:grid-cols-4">
              <a href="/nodes/{data.node?.nodeId}/docker/containers" class="stat-link">
                <div class="text-2xl font-semibold">{data.dockerStats.containersRunning}/{data.dockerStats.containersTotal}</div>
                <div class="text-xs text-muted-foreground">{$messages.nodes.docker.containers}</div>
              </a>
              <a href="/nodes/{data.node?.nodeId}/docker/images" class="stat-link">
                <div class="text-2xl font-semibold">{data.dockerStats.images}</div>
                <div class="text-xs text-muted-foreground">{$messages.nodes.docker.images}</div>
              </a>
              <a href="/nodes/{data.node?.nodeId}/docker/networks" class="stat-link">
                <div class="text-2xl font-semibold">{data.dockerStats.networks}</div>
                <div class="text-xs text-muted-foreground">{$messages.nodes.docker.networks}</div>
              </a>
              <a href="/nodes/{data.node?.nodeId}/docker/volumes" class="stat-link">
                <div class="text-2xl font-semibold">{data.dockerStats.volumes}</div>
                <div class="text-xs text-muted-foreground">{$messages.nodes.docker.volumes}</div>
              </a>
            </div>

            <div class="text-sm text-muted-foreground">
              {$messages.nodes.docker.version} {data.dockerStats.dockerServerVersion || $messages.common.unknown}
              {#if data.dockerStats.volumesSizeBytes > 0}
                · {formatBytes(data.dockerStats.volumesSizeBytes)} {$messages.nodes.docker.volumesSize}
              {/if}
              {#if data.dockerStats.disksUsageBytes > 0}
                · {formatBytes(data.dockerStats.disksUsageBytes)} {$messages.nodes.docker.diskUsage}
              {/if}
            </div>

            {#if form?.error}
              <Alert variant="destructive">
                <AlertDescription>{form.error}</AlertDescription>
              </Alert>
            {/if}

            {#if data.node?.isOnline}
              <div class="flex flex-wrap gap-2">
                <form method="POST" action="?/syncCaddyFiles" use:enhance>
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.rebuildCaddy}</Button>
                </form>
                <form method="POST" action="?/reloadCaddy" use:enhance>
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.reloadCaddy}</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="all" />
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.prune.all}</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="containers" />
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.prune.containers}</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="images" />
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.prune.images}</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="networks" />
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.prune.networks}</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="volumes" />
                  <Button variant="outline" size="sm" type="submit">{$messages.nodes.docker.prune.volumes}</Button>
                </form>
              </div>
            {:else}
              <div class="text-sm text-muted-foreground">{$messages.nodes.docker.nodeOffline}</div>
            {/if}
          </div>
        {:else}
          <div class="text-sm text-muted-foreground">{$messages.nodes.docker.noStats}</div>
        {/if}
      </CardContent>
    </Card>

		<Card>
      <CardHeader>
        <div class="section-header">
          <div class="section-heading">
            <CardTitle class="section-title">{$messages.dashboard.recentTasks}</CardTitle>
          </div>
          {#if data.node}
            <a class="text-sm text-muted-foreground transition-colors hover:text-foreground" href={`/tasks?nodeId=${encodeURIComponent(data.node.nodeId)}`}>
              {$messages.common.viewAll}
            </a>
          {/if}
        </div>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.tasks as task}
            <a
              href={`/tasks/${task.taskId}`}
              class="list-row"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-medium">{task.type}</div>
                  <div class="truncate text-xs text-muted-foreground">{task.serviceName ?? $messages.tasks.nodeLevel}</div>
                </div>
                <Badge variant={taskStatusTone(task.status)}>{taskStatusLabel(task.status, $messages)}</Badge>
              </div>
              <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(task.createdAt)}</div>
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
