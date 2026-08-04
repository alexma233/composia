<script lang="ts">
  import { onMount } from "svelte";

  import { getMessages } from "$lib/i18n";

  const messages = getMessages();

  type DataHandler = (data: string) => void;
  type ResizeHandler = (rows: number, cols: number) => void;

  interface Props {
    active?: boolean;
    content?: string;
    emptyText?: string;
    fixedCols?: number;
    heightClass?: string;
    interactive?: boolean;
    onData?: DataHandler;
    onResize?: ResizeHandler;
  }

  let {
    active = false,
    content = "",
    emptyText = "",
    fixedCols = 0,
    heightClass = "h-[360px]",
    interactive = false,
    onData,
    onResize,
  }: Props = $props();

  let host = $state<HTMLDivElement | null>(null);

  const TERMINAL_RESET = "\x1bc\x1b[3J\x1b[H\x1b[2J";

  let terminal: import("@wterm/dom").WTerm | null = null;
  let scrollScheduled = false;
  let renderedText = "";

  function normalizeTerminalText(value: string): string {
    return value.replace(/\r?\n/g, "\r\n");
  }

  function currentText(): string {
    return normalizeTerminalText(content || emptyText);
  }

  function syncTerminal(force = false) {
    if (!terminal) {
      return;
    }

    const nextText = currentText();
    if (!force && nextText === renderedText) {
      return;
    }

    const canAppend =
      !force &&
      content !== "" &&
      renderedText !== "" &&
      nextText.startsWith(renderedText);
    if (canAppend) {
      terminal.write(nextText.slice(renderedText.length));
    } else {
      terminal.write(TERMINAL_RESET);
      if (nextText) {
        terminal.write(nextText);
      }
    }

    renderedText = nextText;
    scheduleReadOnlyScrollToBottom();
  }

  function scheduleReadOnlyScrollToBottom() {
    if (interactive || scrollScheduled) {
      return;
    }

    scrollScheduled = true;
    requestAnimationFrame(() => {
      scrollScheduled = false;
      if (terminal) {
        terminal.element.scrollTop = terminal.element.scrollHeight;
      }
    });
  }

  function disableReadOnlyInput() {
    if (interactive || !host) {
      return;
    }

    const input = host.querySelector("textarea");
    input?.setAttribute("tabindex", "-1");
    if (input instanceof HTMLTextAreaElement) {
      input.blur();
    }
  }

  onMount(() => {
    let disposed = false;
    const stopReadOnlyFocus = (event: MouseEvent) => {
      if (!interactive) {
        event.stopImmediatePropagation();
      }
    };

    async function setup() {
      const [{ WTerm }] = await Promise.all([
        import("@wterm/dom"),
        import("@wterm/dom/css"),
      ]);
      if (disposed || !host) {
        return;
      }

      terminal = new WTerm(host, {
        cols: fixedCols || undefined,
        autoResize: true,
        cursorBlink: interactive,
        onData: interactive ? (data) => onData?.(data) : () => {},
        onResize: (cols, rows) => onResize?.(rows, cols),
      });

      host.addEventListener("click", stopReadOnlyFocus, { capture: true });
      await terminal.init();
      if (disposed) {
        terminal.destroy();
        terminal = null;
        return;
      }

      disableReadOnlyInput();
      syncTerminal(true);
    }

    void setup();

    return () => {
      disposed = true;
      host?.removeEventListener("click", stopReadOnlyFocus, { capture: true });
      terminal?.destroy();
      terminal = null;
      scrollScheduled = false;
      renderedText = "";
    };
  });

  $effect(() => {
    content;
    emptyText;
    syncTerminal();
  });

  $effect(() => {
    if (active && interactive) {
      terminal?.focus();
    }
  });
</script>

<div
  class={`terminal-surface ${heightClass}`}
  data-fixed-cols={fixedCols || undefined}
  style={fixedCols ? `--terminal-cols: ${fixedCols}` : undefined}
>
  <div
    bind:this={host}
    class="terminal-host h-full"
    aria-label={$messages.common.terminal}
  ></div>
</div>
