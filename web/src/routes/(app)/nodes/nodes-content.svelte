<script lang="ts">
  import { invalidate } from "$app/navigation";
  import { onMount } from "svelte";
  import type { PageData } from "./$types";

  import {
    Alert,
    AlertDescription,
    AlertTitle,
  } from "$lib/components/ui/alert";
  import { Badge } from "$lib/components/ui/badge";
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import { startPolling } from "$lib/refresh";
  import {
    Table,
    TableBody,
    TableCaption,
    TableHead,
    TableHeader,
    TableRow,
  } from "$lib/components/ui/table";
  import { getMessages } from "$lib/i18n";

  const messages = getMessages();
  import NodeRow from "$lib/components/app/node-row.svelte";
  interface Props {
    data: Awaited<PageData["content"]>;
  }

  let { data }: Props = $props();

  onMount(() =>
    startPolling(() => invalidate("app:nodes"), { intervalMs: 15000 }),
  );
</script>

<div class="page-shell">
  <Card>
    <CardHeader>
      <div class="page-header">
        <div class="page-heading">
          <CardTitle class="page-title" level="1"
            >{$messages.nodes.title}</CardTitle
          >
        </div>
        <Badge variant="outline">{data.nodes.length}</Badge>
      </div>

      {#if data.error}
        <Alert variant="destructive">
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
      {/if}
    </CardHeader>

    <CardContent>
      {#if data.error}
        <!-- Load failure is already rendered in the header. -->
      {:else if data.nodes.length}
        <Table>
          <TableCaption class="sr-only"
            >{$messages.nodes.tableCaption}</TableCaption
          >
          <TableHeader>
            <TableRow>
              <TableHead>{$messages.nodes.node}</TableHead>
              <TableHead>{$messages.common.status}</TableHead>
              <TableHead class="w-56">{$messages.nodes.lastHeartbeat}</TableHead
              >
            </TableRow>
          </TableHeader>
          <TableBody>
            {#each data.nodes as node (node.nodeId)}
              <NodeRow {node} />
            {/each}
          </TableBody>
        </Table>
      {:else}
        <div class="empty-state">{$messages.nodes.noNodes}</div>
      {/if}
    </CardContent>
  </Card>
</div>
