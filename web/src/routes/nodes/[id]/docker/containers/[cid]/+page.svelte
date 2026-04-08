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
  import Textarea from '$lib/components/ui/textarea/textarea.svelte';

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
  let terminalInput = $state('');
  let terminalSessionId = $state('');
  let terminalSocket = $state<WebSocket | null>(null);

  $effect(() => {
    if (!data.rawJson) {
      containerData = null;
      parseError = null;
      return;
    }

    try {
      const parsed = JSON.parse(data.rawJson);
      containerData = Array.isArray(parsed) ? parsed[0] : parsed;
      parseError = null;
    } catch (error) {
      parseError = error instanceof Error ? error.message : 'Failed to parse container data';
      containerData = null;
    }
  });

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
        throw new Error(payload.error ?? 'Failed to load logs');
      }
      logs = payload.content ?? '';
    } catch (error) {
      logsError = error instanceof Error ? error.message : 'Failed to load logs';
      logs = '';
    } finally {
      logsLoading = false;
    }
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
      window.location.reload();
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
        body: JSON.stringify({ command, rows: 32, cols: 120 })
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? 'Failed to open terminal');
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
              terminalOutput = `${terminalOutput}\n[session closed]`.trim();
            }
          } catch {
            terminalOutput = `${terminalOutput}${event.data}`;
          }
          return;
        }
        const text = new TextDecoder().decode(event.data instanceof ArrayBuffer ? event.data : new Uint8Array());
        terminalOutput = `${terminalOutput}${text}`;
      };
      socket.onerror = () => {
        terminalError = 'Terminal connection error';
      };
      socket.onclose = () => {
        terminalSocket = null;
      };
      terminalSocket = socket;
    } catch (error) {
      terminalError = error instanceof Error ? error.message : 'Failed to open terminal';
    } finally {
      terminalConnecting = false;
    }
  }

  function sendTerminalInput() {
    if (!terminalSocket || terminalSocket.readyState !== WebSocket.OPEN || !terminalInput) {
      return;
    }
    terminalSocket.send(terminalInput);
    terminalInput = '';
  }

  onMount(() => {
    activeTab = data.initialTab ?? 'info';
    if (activeTab === 'logs') {
      void loadLogs();
    }
    return () => disconnectTerminal();
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
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="page-title">
              {#if containerData}
                {containerData.Name?.replace(/^\//, '') || data.containerId}
              {:else}
                Container
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
          <div class="flex items-center gap-2">
            {#if containerData}
              <Badge variant={getStateVariant(containerData.State?.Status)}>
                {containerData.State?.Status || 'unknown'}
              </Badge>
            {/if}
            <Button variant="outline" size="sm" onclick={() => void queueContainerAction('start')} disabled={actionBusy !== '' || containerData?.State?.Status?.toLowerCase() === 'running'}>Start</Button>
            <Button variant="outline" size="sm" onclick={() => void queueContainerAction('stop')} disabled={actionBusy !== '' || containerData?.State?.Status?.toLowerCase() !== 'running'}>Stop</Button>
            <Button variant="outline" size="sm" onclick={() => void queueContainerAction('restart')} disabled={actionBusy !== ''}>Restart</Button>
            <a href="/nodes/{data.nodeId}/docker/containers" class="text-sm text-muted-foreground hover:underline">
              Back to containers
            </a>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        {#if data.error}
          <Alert variant="destructive">
            <AlertTitle>Load failed</AlertTitle>
            <AlertDescription>{data.error}</AlertDescription>
          </Alert>
        {:else if parseError}
          <Alert variant="destructive">
            <AlertTitle>Parse failed</AlertTitle>
            <AlertDescription>Failed to parse container data: {parseError}</AlertDescription>
          </Alert>
        {:else if containerData}
          <Tabs bind:value={activeTab} class="w-full">
            <TabsList class="mb-4">
              <TabsTrigger value="info">Info</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="terminal">Terminal</TabsTrigger>
              <TabsTrigger value="config">Config</TabsTrigger>
              <TabsTrigger value="env">Environment</TabsTrigger>
              <TabsTrigger value="network">Network</TabsTrigger>
              <TabsTrigger value="volumes">Volumes</TabsTrigger>
              <TabsTrigger value="labels">Labels</TabsTrigger>
              <TabsTrigger value="raw">JSON</TabsTrigger>
            </TabsList>

            <TabsContent value="info" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">General</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">ID</span>
                      <code class="text-xs bg-muted px-1 py-0.5 rounded">{containerData.Id?.substring(0, 12)}</code>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Name</span>
                      <span>{containerData.Name?.replace(/^\//, '') || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Image</span>
                      <span class="truncate max-w-[200px]" title={containerData.Config?.Image}>
                        {containerData.Config?.Image || '-'}
                      </span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Platform</span>
                      <span>{containerData.Platform || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Driver</span>
                      <span>{containerData.Driver || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Created</span>
                      <span>{formatDate(containerData.Created)}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Runtime</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Status</span>
                      <Badge variant={getStateVariant(containerData.State?.Status)}>
                        {containerData.State?.Status || 'unknown'}
                      </Badge>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Running</span>
                      <span>{containerData.State?.Running ? 'Yes' : 'No'}</span>
                    </div>
                    {#if containerData.State?.Running}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Uptime</span>
                        <span>{formatDuration(containerData.State?.StartedAt)}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Started</span>
                        <span>{formatDate(containerData.State?.StartedAt)}</span>
                      </div>
                    {:else}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Exit Code</span>
                        <span>{containerData.State?.ExitCode ?? '-'}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Finished</span>
                        <span>{formatDate(containerData.State?.FinishedAt)}</span>
                      </div>
                    {/if}
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Restart Count</span>
                      <span>{containerData.RestartCount || 0}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">OOM Killed</span>
                      <span>{containerData.State?.OOMKilled ? 'Yes' : 'No'}</span>
                    </div>
                  </CardContent>
                </Card>
              </div>

              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Process</CardTitle>
                </CardHeader>
                <CardContent class="space-y-2 text-sm">
                  <div class="grid gap-4 md:grid-cols-3">
                    <div>
                      <span class="text-muted-foreground">Command</span>
                      <code class="block mt-1 text-xs bg-muted p-2 rounded break-all">
                        {containerData.Config?.Cmd?.join(' ') || '-'}
                      </code>
                    </div>
                    <div>
                      <span class="text-muted-foreground">Entrypoint</span>
                      <code class="block mt-1 text-xs bg-muted p-2 rounded break-all">
                        {containerData.Config?.Entrypoint?.join(' ') || '-'}
                      </code>
                    </div>
                    <div>
                      <span class="text-muted-foreground">Working Directory</span>
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
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <CardTitle class="text-base">Container Logs</CardTitle>
                      <CardDescription>Fetch recent stdout and stderr output.</CardDescription>
                    </div>
                    <div class="flex items-center gap-2">
                      <Input bind:value={logTail} class="w-24" />
                      <Button variant="outline" size="sm" onclick={() => void loadLogs()} disabled={logsLoading}>Refresh</Button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  {#if logsError}
                    <Alert variant="destructive">
                      <AlertDescription>{logsError}</AlertDescription>
                    </Alert>
                  {/if}
                  <pre class="code-surface min-h-[320px] max-h-[560px] overflow-auto break-all text-xs">{logsLoading ? 'Loading logs...' : (logs || 'No logs returned.')}</pre>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="terminal" class="space-y-4">
              <Card>
                <CardHeader class="pb-3">
                  <CardTitle class="text-base">Terminal</CardTitle>
                  <CardDescription>Open an interactive exec session through the controller tunnel.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                  <div class="flex flex-wrap items-center gap-2">
                    <Input bind:value={terminalCommand} placeholder="/bin/sh" class="min-w-[220px] flex-1" />
                    <Button onclick={() => void connectTerminal()} disabled={terminalConnecting}>
                      {terminalSocket ? 'Reconnect' : 'Connect'}
                    </Button>
                    <Button variant="outline" onclick={disconnectTerminal} disabled={!terminalSocket}>Disconnect</Button>
                  </div>

                  {#if terminalError}
                    <Alert variant="destructive">
                      <AlertDescription>{terminalError}</AlertDescription>
                    </Alert>
                  {/if}

                  <Textarea readonly value={terminalOutput || (terminalConnecting ? 'Connecting terminal...' : 'Connect to start a shell session.')} class="min-h-[320px] font-mono text-xs" />

                  <div class="flex gap-2">
                    <Input bind:value={terminalInput} placeholder="Type command input and press Send" onkeydown={(event: KeyboardEvent) => { if (event.key === 'Enter') { event.preventDefault(); sendTerminalInput(); } }} />
                    <Button variant="outline" onclick={sendTerminalInput} disabled={!terminalSocket || !terminalInput}>Send</Button>
                  </div>

                  {#if terminalSessionId}
                    <div class="text-xs text-muted-foreground">Session {terminalSessionId}</div>
                  {/if}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="config" class="space-y-4">
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Configuration</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Hostname</span>
                      <span>{containerData.Config?.Hostname || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Domainname</span>
                      <span>{containerData.Config?.Domainname || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">User</span>
                      <span>{containerData.Config?.User || 'root'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Attach Stdin</span>
                      <span>{containerData.Config?.AttachStdin ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Attach Stdout</span>
                      <span>{containerData.Config?.AttachStdout ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Attach Stderr</span>
                      <span>{containerData.Config?.AttachStderr ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">TTY</span>
                      <span>{containerData.Config?.Tty ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Open Stdin</span>
                      <span>{containerData.Config?.OpenStdin ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Stdin Once</span>
                      <span>{containerData.Config?.StdinOnce ? 'Yes' : 'No'}</span>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader class="pb-3">
                    <CardTitle class="text-base">Host Configuration</CardTitle>
                  </CardHeader>
                  <CardContent class="space-y-2 text-sm">
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Network Mode</span>
                      <span>{containerData.HostConfig?.NetworkMode || 'default'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Privileged</span>
                      <span>{containerData.HostConfig?.Privileged ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Auto Remove</span>
                      <span>{containerData.HostConfig?.AutoRemove ? 'Yes' : 'No'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted-foreground">Readonly Rootfs</span>
                      <span>{containerData.HostConfig?.ReadonlyRootfs ? 'Yes' : 'No'}</span>
                    </div>
                    {#if containerData.HostConfig?.Memory}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Memory Limit</span>
                        <span>{formatBytes(containerData.HostConfig.Memory)}</span>
                      </div>
                    {/if}
                    {#if containerData.HostConfig?.CpuShares}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">CPU Shares</span>
                        <span>{containerData.HostConfig.CpuShares}</span>
                      </div>
                    {/if}
                    {#if containerData.HostConfig?.RestartPolicy?.Name}
                      <div class="flex justify-between">
                        <span class="text-muted-foreground">Restart Policy</span>
                        <span>
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
                        <div class="flex gap-2 text-sm font-mono text-xs border-b border-border/50 last:border-0 py-1.5">
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
                        <div class="flex items-center gap-3 text-sm border-b border-border/50 last:border-0 py-2">
                          <Badge variant="secondary">{containerPort}</Badge>
                          <span class="text-muted-foreground">→</span>
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
                  <div class="flex justify-between">
                    <span class="text-muted-foreground">Gateway</span>
                    <span>{containerData.NetworkSettings?.Gateway || '-'}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-muted-foreground">IPAddress</span>
                    <span>{containerData.NetworkSettings?.IPAddress || '-'}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-muted-foreground">MacAddress</span>
                    <span>{containerData.NetworkSettings?.MacAddress || '-'}</span>
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
                          <div class="flex items-center gap-2 mb-1">
                            <Badge variant="outline">{mount.Type}</Badge>
                            <Badge variant="secondary">{mount.Mode || 'rw'}</Badge>
                          </div>
                          <div class="grid gap-1 text-sm">
                            <div class="flex gap-2">
                              <span class="text-muted-foreground w-16 shrink-0">Source:</span>
                              <code class="text-xs bg-muted px-1 py-0.5 rounded break-all">{mount.Source}</code>
                            </div>
                            <div class="flex gap-2">
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
                        <div class="flex gap-2 text-sm border-b border-border/50 last:border-0 py-1.5">
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
                  <pre class="code-surface max-h-[600px] overflow-auto break-all">{JSON.stringify(containerData, null, 2)}</pre>
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
