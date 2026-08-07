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
  let effectiveCols = $state(0);
  let fixedResizeObserver: ResizeObserver | null = null;

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

  function terminalRowsForHost(): number {
    if (!terminal) {
      return 24;
    }

    const style = getComputedStyle(terminal.element);
    const rowHeight = Number.parseFloat(style.getPropertyValue("--term-row-height")) || 17;
    const verticalPadding =
      Number.parseFloat(style.paddingTop) + Number.parseFloat(style.paddingBottom);
    return Math.max(1, Math.floor((terminal.element.clientHeight - verticalPadding) / rowHeight));
  }

  function terminalColumnsForHost(): number {
    if (!terminal || !fixedCols) {
      return 0;
    }

    const style = getComputedStyle(terminal.element);
    const horizontalPadding =
      Number.parseFloat(style.paddingLeft) + Number.parseFloat(style.paddingRight);
    const probe = document.createElement("span");
    probe.textContent = "0";
    probe.style.cssText = "position:absolute;visibility:hidden;width:1ch";
    terminal.element.append(probe);
    const charWidth = probe.getBoundingClientRect().width;
    probe.remove();
    if (!charWidth) {
      return fixedCols;
    }

    return Math.max(
      fixedCols,
      Math.floor((terminal.element.clientWidth - horizontalPadding) / charWidth),
    );
  }

  function resizeFixedTerminal() {
    if (!terminal || !fixedCols) {
      return;
    }

    const cols = terminalColumnsForHost();
    const rows = terminalRowsForHost();
    const colsChanged = terminal.cols !== cols;
    if (colsChanged || terminal.rows !== rows) {
      terminal.resize(cols, rows);
    }
    effectiveCols = cols;
    if (colsChanged && !interactive) {
      renderedText = "";
      syncTerminal(true);
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

      effectiveCols = fixedCols;

      terminal = new WTerm(host, {
        cols: fixedCols || undefined,
        autoResize: !fixedCols,
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

      if (fixedCols) {
        resizeFixedTerminal();
        fixedResizeObserver = new ResizeObserver(() => {
          resizeFixedTerminal();
        });
        fixedResizeObserver.observe(terminal.element);
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
      fixedResizeObserver?.disconnect();
      fixedResizeObserver = null;
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
    if (fixedCols) {
      effectiveCols = fixedCols;
      resizeFixedTerminal();
    }
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
  style={fixedCols ? `--terminal-cols: ${effectiveCols}` : undefined}
>
  <div
    bind:this={host}
    class="terminal-host h-full"
    aria-label={$messages.common.terminal}
  ></div>
</div>
