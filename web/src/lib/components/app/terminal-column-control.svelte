<script lang="ts">
  import { getMessages } from "$lib/i18n";
  import {
    setTerminalColumns,
    terminalColumnOptions,
    terminalColumns,
    type TerminalColumns,
  } from "$lib/preferences";
  import * as Select from "$lib/components/ui/select";

  const messages = getMessages();

  function changeColumns(value: string | undefined) {
    const columns = Number(value) as TerminalColumns;
    if (terminalColumnOptions.includes(columns)) {
      setTerminalColumns(columns);
    }
  }
</script>

<Select.Root type="single" value={String($terminalColumns)} onValueChange={changeColumns}>
  <Select.Trigger
    class="w-[6.5rem]"
    aria-label={$messages.common.terminalColumns}
  >
    {$terminalColumns} {$messages.common.terminalColumns}
  </Select.Trigger>
  <Select.Content>
    {#each terminalColumnOptions as columns}
      <Select.Item value={String(columns)}>{columns}</Select.Item>
    {/each}
  </Select.Content>
</Select.Root>
