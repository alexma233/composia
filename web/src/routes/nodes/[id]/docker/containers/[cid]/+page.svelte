<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
  import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import XtermSurface from '$lib/components/app/xterm-surface.svelte';
  import { messages } from '$lib/i18n';
  import { startPolling } from '$lib/refresh';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let containerData = $state<any>(null);
  let parseError = $state<string | null>(null);
  let activeTab = $state('info');
  let logs = $state('');
  let logsLoading = $state(false);
  let logsError = $state('');
  let logTail = $state('200');
  let actionBusy = $state('');
  let terminalCommand = $state('/bin/sh');
  let terminalConnecting = $state(false);
  let terminalError = $state('');
  let terminalOutput = $state('');
  let terminalSessionId = $state('');
  let terminalSocket = $state<WebSocket | null>(null);
  let terminalRows = $state(32);
  let terminalCols = $state(120);
  let stopActionRefreshHandle = $state<null | (() => void)>(null);

  $effect(() => {
    applyContainerRawJson(data.rawJson);
  });

  function applyContainerRawJson(rawJson: string | null) {
    if (!rawJson) {
      containerData = null;
      parseError = null;
      return;
    }

    try {
      const parsed = JSON.parse(rawJson);
      containerData = Array.isArray(parsed) ? parsed[0] : parsed;
      parseError = null;
    } catch (error) {
      parseError = error instanceof Error ? error.message : $messages.error.parseFailed;
      containerData = null;
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function formatDate(timestamp: string): string {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleString();
  }

  function formatDuration(startedAt: string): string {
    if (!startedAt) return '-';
    const start = new Date(startedAt);
    const now = new Date();
    const diff = Math.floor((now.getTime() - start.getTime()) / 1000);

    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
    return `${Math.floor(diff / 86400)}d`;
  }

  function getStateVariant(state: string): BadgeVariant {
    const s = (state || '').toLowerCase();
    if (s === 'running') return 'default';
    if (s === 'created' || s === 'starting') return 'outline';
    if (s === 'paused') return 'secondary';
    if (s === 'restarting' || s === 'unhealthy') return 'outline';
    if (s === 'exited' || s === 'dead' || s === 'removing') return 'destructive';
    return 'default';
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0 || !bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  async function loadLogs() {
    logsLoading = true;
    logsError = '';
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(data.containerId)}/logs?tail=${encodeURIComponent(logTail)}`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? $messages.docker.containers.logs.loadFailed);
      }
      logs = payload.content ?? '';
    } catch (error) {
      logsError = error instanceof Error ? error.message : $messages.docker.containers.logs.loadFailed;
      logs = '';
    } finally {
      logsLoading = false;
    }
  }

  async function refreshContainerDetails() {
    const response = await fetch(
      `/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(data.containerId)}`,
    );
    const payload = await response.json();
    if (!response.ok) {
      throw new Error(payload.error ?? 'Failed to inspect container');
    }

    applyContainerRawJson(payload.rawJson ?? null);
  }

  function isTerminalTaskStatus(status: string | undefined) {
    return status === 'succeeded' || status === 'failed' || status === 'cancelled';
  }

  function stopActionRefresh() {
    stopActionRefreshHandle?.();
    stopActionRefreshHandle = null;
  }

  function startActionRefresh(taskId: string) {
    stopActionRefresh();

    stopActionRefreshHandle = startPolling(async () => {
      const response = await fetch(`/tasks/${encodeURIComponent(taskId)}`);
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? 'Failed to load task detail.');
      }

      if (isTerminalTaskStatus(payload.task?.status)) {
        await refreshContainerDetails();
        return false;
      }

      return true;
    }, {
      intervalMs: 2500,
      errorIntervalMs: 4000,
      initialDelayMs: 1200,
    });
  }

  async function queueContainerAction(action: 'start' | 'stop' | 'restart') {
    actionBusy = action;
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(data.containerId)}/actions/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? `Failed to ${action} container`);
      }
      toast.success(`${action} queued: ${payload.taskId?.slice(0, 12) ?? 'task'}`);
      if (payload.taskId) {
        startActionRefresh(payload.taskId);
      } else {
        await refreshContainerDetails();
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : `Failed to ${action} container.`);
    } finally {
      actionBusy = '';
    }
  }

  function disconnectTerminal() {
    terminalSocket?.close();
    terminalSocket = null;
    terminalSessionId = '';
  }

  function appendTerminalOutput(value: string) {
    terminalOutput = `${terminalOutput}${value}`;
  }

  function appendTerminalNotice(value: string) {
    const prefix = terminalOutput && !terminalOutput.endsWith('\n') ? '\n' : '';
    terminalOutput = `${terminalOutput}${prefix}${value}\n`;
  }

  async function connectTerminal() {
    terminalConnecting = true;
    terminalError = '';
    terminalOutput = '';
    disconnectTerminal();
    try {
      const command = terminalCommand.trim() ? terminalCommand.trim().split(/\s+/) : [];
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(data.containerId)}/exec`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command, rows: terminalRows, cols: terminalCols })
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? $messages.docker.containers.terminal.connectionFailed);
      }
      terminalSessionId = payload.sessionId ?? '';
      const socket = new WebSocket(payload.websocketUrl);
      socket.binaryType = 'arraybuffer';
      socket.onmessage = (event) => {
        if (typeof event.data === 'string') {
          try {
            const message = JSON.parse(event.data) as { type?: string; message?: string };
            if (message.type === 'error' && message.message) {
              terminalError = message.message;
            }
            if (message.type === 'closed') {
              appendTerminalNotice($messages.docker.containers.terminal.sessionClosed);
            }
          } catch {
            appendTerminalOutput(event.data);
          }
          return;
        }
        const text = new TextDecoder().decode(event.data instanceof ArrayBuffer ? event.data : new Uint8Array());
        appendTerminalOutput(text);
      };
      socket.onerror = () => {
        terminalError = $messages.docker.containers.terminal.connectionFailed;
      };
      socket.onclose = () => {
        terminalSocket = null;
      };
      terminalSocket = socket;
    } catch (error) {
      terminalError = error instanceof Error ? error.message : $messages.docker.containers.terminal.connectionFailed;
    } finally {
      terminalConnecting = false;
    }
  }

  function sendTerminalData(data: string) {
    if (!terminalSocket || terminalSocket.readyState !== WebSocket.OPEN) {
      return;
    }
    terminalSocket.send(data);
  }

  function resizeTerminal(rows: number, cols: number) {
    terminalRows = rows;
    terminalCols = cols;

    if (!terminalSocket || terminalSocket.readyState !== WebSocket.OPEN) {
      return;
    }

    terminalSocket.send(JSON.stringify({ type: 'resize', rows, cols }));
  }

  onMount(() => {
    activeTab = data.initialTab ?? 'info';
    if (activeTab === 'logs') {
      void loadLogs();
    }
    return () => {
      stopActionRefresh();
      disconnectTerminal();
    };
  });

  $effect(() => {
    if (activeTab === 'logs' && !logs && !logsLoading) {
      void loadLogs();
    }
  });
</script>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-1">
            <CardTitle class="page-title">
              {#if containerData}
                {containerData.Name?.replace(/^\//, '') || data.containerId}
              {:else}
                {$messages.docker.containers.container}
              {/if}
            </CardTitle>
            <CardDescription class="page-description">
              {#if containerData}
                <code class="text-xs bg-muted px-1 py-0.5 rounded">{containerData.Id}</code>
              {:else}
                {data.containerId}
              {/if}
            </CardDescription>
          </div>
          <div class="flex w-full flex-col gap-2 sm:w-auto sm:items-end">
            {#if containerData}
              <Badge variant={getStateVariant(containerData.State?.Status)} class="w-fit">
                {containerData.State?.Status || $messages.common.unknown}
              </Badge>
            {/if}
            <div class="flex flex-wrap gap-2 sm:justify-end">
              <Button class="flex-1 sm:flex-none" variant="outline" size="sm" onclick={() => void queueContainerAction('start')} disabled={actionBusy !== '' || containerData?.State?.Status?.toLowerCase() === 'running'}>{$messages.docker.containers.start}</Button>
              <Button class="flex-1 sm:flex-none" variant="outline" size="sm" onclick={() => void queueContainerAction('stop')} disabled={actionBusy !== '' || containerData?.State?.Status?.toLowerCase() !== 'running'}>{$messages.docker.containers.stop}</Button>
              <Button class="flex-1 sm:flex-none" variant="outline" size="sm" onclick={() => void queueContainerAction('restart')} disabled={actionBusy !== ''}>{$messages.docker.containers.restart}</Button>
            </div>
            <a href="/nodes/{data.nodeId}/docker/containers" class="text-sm text-muted-foreground hover:underline sm:text-right">
              {$messages.docker.containers.backToContainers}
            </a>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        {#if data.error}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
            <AlertDescription>{data.error}</AlertDescription>
          </Alert>
        {:else if parseError}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.parseFailed}</AlertTitle>
            <AlertDescription>{$messages.error.parseFailed}: {parseError}</AlertDescription>
          </Alert>
        {:else if containerData}
          <Tabs bind:value={activeTab} class="w-full">
            <div class="mb-4 overflow-x-auto pb-1 scrollbar-none">
              <TabsList class="min-w-max">
                <TabsTrigger value="info">{$messages.docker.containers.info}</TabsTrigger>
                <TabsTrigger value="logs">{$messages.docker.containers.logsLabel}</TabsTrigger>
                <TabsTrigger value="terminal">{$messages.docker.containers.terminalLabel}</TabsTrigger>
                <TabsTrigger value="config">{$messages.docker.containers.config}</TabsTrigger>
                <TabsTrigger value="env">{$messages.docker.containers.environment}</TabsTrigger>
                <TabsTrigger value="network">{$messages.docker.containers.network}</TabsTrigger>
                <TabsTrigger value="volumes">{$messages.docker.containers.volumes}</TabsTrigger>
                <TabsTrigger value="labels">{$messages.docker.containers.labels}</TabsTrigger>
                <TabsTrigger value="raw">{$messages.docker.containers.json}</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.containers.general}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">ID</span>
                      <code class="text-xs bg-muted px-1 py-0.5 rounded">{containerData.Id?.substring(0, 12)}</code>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.common.name}</span>
                      <span class="break-all sm:text-right">{containerData.Name?.replace(/^\//, '') || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.image}</span>
                      <span class="break-all sm:max-w-[20rem] sm:text-right" title={containerData.Config?.Image}>
                        {containerData.Config?.Image || '-'}
                      </span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.platform}</span>
                      <span class="break-all sm:text-right">{containerData.Platform || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.driver}</span>
                      <span class="break-all sm:text-right">{containerData.Driver || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.created}</span>
                      <span class="sm:text-right">{formatDate(containerData.Created)}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.containers.runtime}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.common.status}</span>
                      <Badge variant={getStateVariant(containerData.State?.Status)}>
                        {containerData.State?.Status || $messages.common.unknown}
                      </Badge>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.running}</span>
                      <span class="sm:text-right">{containerData.State?.Running ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                    {#if containerData.State?.Running}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.containers.uptime}</span>
                        <span class="sm:text-right">{formatDuration(containerData.State?.StartedAt)}</span>
                      </div>
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.containers.started}</span>
                        <span class="sm:text-right">{formatDate(containerData.State?.StartedAt)}</span>
                      </div>
                    {:else}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.containers.exitCode}</span>
                        <span class="sm:text-right">{containerData.State?.ExitCode ?? '-'}</span>
                      </div>
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">{$messages.docker.containers.finished}</span>
                        <span class="sm:text-right">{formatDate(containerData.State?.FinishedAt)}</span>
                      </div>
                    {/if}
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.restartCount}</span>
                      <span class="sm:text-right">{containerData.RestartCount || 0}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.oomKilled}</span>
                      <span class="sm:text-right">{containerData.State?.OOMKilled ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                  </CardContent>
                </Card>
              </div>

              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">{$messages.docker.containers.process}</CardTitle>
                </CardHeader>
                <CardContent class="space-y-2 text-sm">
                  <div class="grid gap-4 md:grid-cols-3">
                    <div>
                      <span class="text-muted-foreground">{$messages.docker.containers.command}</span>
                      <code class="block mt-1 text-xs bg-muted p-2 rounded break-all">
                        {containerData.Config?.Cmd?.join(' ') || '-'}
                      </code>
                    </div>
                    <div>
                      <span class="text-muted-foreground">{$messages.docker.containers.entrypoint}</span>
                      <code class="block mt-1 text-xs bg-muted p-2 rounded break-all">
                        {containerData.Config?.Entrypoint?.join(' ') || '-'}
                      </code>
                    </div>
                    <div>
                      <span class="text-muted-foreground">{$messages.docker.containers.workingDir}</span>
                      <code class="block mt-1 text-xs bg-muted p-2 rounded break-all">
                        {containerData.Config?.WorkingDir || '/'}
                      </code>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="logs" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <CardTitle class="text-base">{$messages.docker.containers.containerLogs}</CardTitle>
                      <CardDescription>{$messages.docker.containers.logs.description}</CardDescription>
                    </div>
                    <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center">
                      <Input bind:value={logTail} class="w-full sm:w-24" />
                      <Button variant="outline" size="sm" onclick={() => void loadLogs()} disabled={logsLoading}>{$messages.docker.containers.logs.refresh}</Button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  {#if logsError}
                    <Alert variant="destructive">
                      <AlertDescription>{logsError}</AlertDescription>
                    </Alert>
                  {/if}

                  <XtermSurface
                    active={activeTab === 'logs'}
                    content={logsLoading ? $messages.common.loadingWithDots : logs}
                    emptyText={$messages.docker.containers.logs.noLogs}
                    heightClass="h-[360px] sm:h-[560px]"
                  />
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="terminal" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">{$messages.docker.containers.terminal.title}</CardTitle>
                  <CardDescription>{$messages.docker.containers.terminal.description}</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                  <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <Input bind:value={terminalCommand} placeholder="/bin/sh" class="w-full sm:min-w-[220px] sm:flex-1" />
                    <Button class="w-full sm:w-auto" onclick={() => void connectTerminal()} disabled={terminalConnecting}>
                      {terminalSocket ? $messages.docker.containers.terminal.reconnect : $messages.docker.containers.terminal.connect}
                    </Button>
                    <Button class="w-full sm:w-auto" variant="outline" onclick={disconnectTerminal} disabled={!terminalSocket}>{$messages.docker.containers.terminal.disconnect}</Button>
                  </div>

                  {#if terminalError}
                    <Alert variant="destructive">
                      <AlertDescription>{terminalError}</AlertDescription>
                    </Alert>
                  {/if}

                  <XtermSurface
                    active={activeTab === 'terminal'}
                    content={terminalOutput}
                    emptyText={terminalConnecting ? $messages.docker.containers.terminal.connecting : (terminalSocket ? '' : $messages.docker.containers.terminal.description)}
                    heightClass="h-[300px] sm:h-[380px]"
                    interactive={true}
                    onData={sendTerminalData}
                    onResize={resizeTerminal}
                  />

                  {#if terminalSessionId}
                    <div class="text-xs text-muted-foreground">{$messages.docker.containers.session} {terminalSessionId}</div>
                  {/if}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="config" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">{$messages.docker.containers.configuration}</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.hostname}</span>
                      <span class="break-all sm:text-right">{containerData.Config?.Hostname || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.domainname}</span>
                      <span class="break-all sm:text-right">{containerData.Config?.Domainname || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.user}</span>
                      <span class="break-all sm:text-right">{containerData.Config?.User || $messages.common.root}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.attachStdin}</span>
                      <span class="sm:text-right">{containerData.Config?.AttachStdin ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.attachStdout}</span>
                      <span class="sm:text-right">{containerData.Config?.AttachStdout ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.attachStderr}</span>
                      <span class="sm:text-right">{containerData.Config?.AttachStderr ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.tty}</span>
                      <span class="sm:text-right">{containerData.Config?.Tty ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.openStdin}</span>
                      <span class="sm:text-right">{containerData.Config?.OpenStdin ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">{$messages.docker.containers.stdinOnce}</span>
                      <span class="sm:text-right">{containerData.Config?.StdinOnce ? $messages.docker.containers.yes : $messages.docker.containers.no}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Host Configuration</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">Network Mode</span>
                      <span class="break-all sm:text-right">{containerData.HostConfig?.NetworkMode || 'default'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">Privileged</span>
                      <span class="sm:text-right">{containerData.HostConfig?.Privileged ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">Auto Remove</span>
                      <span class="sm:text-right">{containerData.HostConfig?.AutoRemove ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">Readonly Rootfs</span>
                      <span class="sm:text-right">{containerData.HostConfig?.ReadonlyRootfs ? 'Yes' : 'No'}</span>
                    </div>
                    {#if containerData.HostConfig?.Memory}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">Memory Limit</span>
                        <span class="sm:text-right">{formatBytes(containerData.HostConfig.Memory)}</span>
                      </div>
                    {/if}
                    {#if containerData.HostConfig?.CpuShares}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <span class="text-muted-foreground">CPU Shares</span>
                        <span class="sm:text-right">{containerData.HostConfig.CpuShares}</span>
                      </div>
                    {/if}
                    {#if containerData.HostConfig?.RestartPolicy?.Name}
                      <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
                        <span class="text-muted-foreground">Restart Policy</span>
                        <span class="break-all sm:text-right">
                          {containerData.HostConfig.RestartPolicy.Name}
                          {#if containerData.HostConfig.RestartPolicy.MaximumRetryCount > 0}
                            (max {containerData.HostConfig.RestartPolicy.MaximumRetryCount})
                          {/if}
                        </span>
                      </div>
                    {/if}
                  </CardContent>
                </Card>
              </div>
            </TabsContent>

            <TabsContent value="env" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Environment Variables</CardTitle>
                </CardHeader>
                <CardContent>
                  {#if containerData.Config?.Env && containerData.Config.Env.length > 0}
                    <div class="space-y-1">
                      {#each containerData.Config.Env as env}
                        {@const [key, ...valueParts] = env.split('=')}
                        {@const value = valueParts.join('=')}
                        <div class="flex flex-col gap-1 border-b border-border/50 py-1.5 font-mono text-xs last:border-0 sm:flex-row sm:gap-2">
                          <span class="text-foreground font-medium shrink-0">{key}=</span>
                          <span class="text-muted-foreground break-all">{value}</span>
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="text-sm text-muted-foreground">No environment variables</div>
                  {/if}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="network" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Port Bindings</CardTitle>
                </CardHeader>
                <CardContent>
                  {#if containerData.HostConfig?.PortBindings && Object.keys(containerData.HostConfig.PortBindings).length > 0}
                    <div class="space-y-2">
                      {#each Object.entries(containerData.HostConfig.PortBindings) as [containerPort, bindings]}
                        {@const typedBindings = (bindings || []) as Array<{HostIp?: string; HostPort?: string}>}
                        <div class="flex flex-col gap-2 border-b border-border/50 py-2 text-sm last:border-0 sm:flex-row sm:items-center sm:gap-3">
                          <div class="flex items-center gap-3">
                            <Badge variant="secondary">{containerPort}</Badge>
                            <span class="text-muted-foreground">→</span>
                          </div>
                          <div class="flex flex-wrap gap-2">
                            {#each typedBindings as binding}
                              <span class="font-mono text-xs">
                                {binding.HostIp || '0.0.0.0'}:{binding.HostPort}
                              </span>
                            {/each}
                          </div>
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="text-sm text-muted-foreground">No port bindings</div>
                  {/if}
                </CardContent>
              </Card>

              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Exposed Ports</CardTitle>
                </CardHeader>
                <CardContent>
                  {#if containerData.Config?.ExposedPorts && Object.keys(containerData.Config.ExposedPorts).length > 0}
                    <div class="flex flex-wrap gap-2">
                      {#each Object.keys(containerData.Config.ExposedPorts) as port}
                        <Badge variant="outline">{port}</Badge>
                      {/each}
                    </div>
                  {:else}
                    <div class="text-sm text-muted-foreground">No exposed ports</div>
                  {/if}
                </CardContent>
              </Card>

              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Network Settings</CardTitle>
                </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">Gateway</span>
                      <span class="break-all sm:text-right">{containerData.NetworkSettings?.Gateway || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">IPAddress</span>
                      <span class="break-all sm:text-right">{containerData.NetworkSettings?.IPAddress || '-'}</span>
                    </div>
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <span class="text-muted-foreground">MacAddress</span>
                      <span class="break-all sm:text-right">{containerData.NetworkSettings?.MacAddress || '-'}</span>
                    </div>
                  </CardContent>
                </Card>
            </TabsContent>

            <TabsContent value="volumes" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Mounts</CardTitle>
                </CardHeader>
                <CardContent>
                  {#if containerData.Mounts && containerData.Mounts.length > 0}
                    <div class="space-y-3">
                      {#each containerData.Mounts as mount}
                        <div class="border-b border-border/50 last:border-0 pb-3 last:pb-0">
                          <div class="mb-1 flex flex-wrap items-center gap-2">
                            <Badge variant="outline">{mount.Type}</Badge>
                            <Badge variant="secondary">{mount.Mode || 'rw'}</Badge>
                          </div>
                          <div class="grid gap-1 text-sm">
                            <div class="flex flex-col gap-1 sm:flex-row sm:gap-2">
                              <span class="text-muted-foreground w-16 shrink-0">Source:</span>
                              <code class="text-xs bg-muted px-1 py-0.5 rounded break-all">{mount.Source}</code>
                            </div>
                            <div class="flex flex-col gap-1 sm:flex-row sm:gap-2">
                              <span class="text-muted-foreground w-16 shrink-0">Target:</span>
                              <code class="text-xs bg-muted px-1 py-0.5 rounded break-all">{mount.Destination}</code>
                            </div>
                          </div>
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="text-sm text-muted-foreground">No mounts</div>
                  {/if}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="labels" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Container Labels</CardTitle>
                </CardHeader>
                <CardContent>
                  {#if containerData.Config?.Labels && Object.keys(containerData.Config.Labels).length > 0}
                    <div class="space-y-1">
                      {#each Object.entries(containerData.Config.Labels) as [key, value]}
                        <div class="flex flex-col gap-1 border-b border-border/50 py-1.5 text-sm last:border-0 sm:flex-row sm:gap-2">
                          <code class="text-xs bg-muted px-1 py-0.5 rounded shrink-0">{key}</code>
                          <span class="text-muted-foreground break-all">{value}</span>
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="text-sm text-muted-foreground">No labels</div>
                  {/if}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="raw">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Raw JSON</CardTitle>
                  <CardDescription>
                    Full container inspection data in JSON format
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <pre class="code-surface max-h-[360px] overflow-auto break-all sm:max-h-[600px]">{JSON.stringify(containerData, null, 2)}</pre>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        {:else}
          <div class="text-sm text-muted-foreground">Loading...</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
