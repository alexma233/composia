<script lang="ts">
  import { onMount } from 'svelte';

  type DataHandler = (data: string) => void;
  type ResizeHandler = (rows: number, cols: number) => void;
  type Disposable = { dispose(): void };

  interface Props {
    active?: boolean;
    content?: string;
    emptyText?: string;
    heightClass?: string;
    interactive?: boolean;
    onData?: DataHandler;
    onResize?: ResizeHandler;
  }

  let {
    active = false,
    content = '',
    emptyText = '',
    heightClass = 'h-[360px]',
    interactive = false,
    onData,
    onResize
  }: Props = $props();

  let host = $state<HTMLDivElement | null>(null);

  let terminal: import('@xterm/xterm').Terminal | null = null;
  let fitAddon: import('@xterm/addon-fit').FitAddon | null = null;
  let dataListener: Disposable | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let themeObserver: MutationObserver | null = null;
  let fitScheduled = false;
  let renderedText = '';

  function normalizeTerminalText(value: string): string {
    return value.replace(/\r?\n/g, '\r\n');
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

    const canAppend = !force && content !== '' && renderedText !== '' && nextText.startsWith(renderedText);
    if (canAppend) {
      terminal.write(nextText.slice(renderedText.length));
    } else {
      terminal.reset();
      if (nextText) {
        terminal.write(nextText);
      }
    }

    renderedText = nextText;
    if (!interactive) {
      terminal.scrollToBottom();
    }
  }

  function applyTheme() {
    if (!terminal || !host) {
      return;
    }

    const styles = getComputedStyle(host);
    terminal.options.theme = {
      background: styles.backgroundColor,
      foreground: styles.color,
      cursor: styles.color,
      cursorAccent: styles.backgroundColor,
      selectionBackground: 'rgba(127, 127, 127, 0.3)'
    };
  }

  function fitTerminal() {
    if (!terminal || !fitAddon || !host || host.clientWidth === 0 || host.clientHeight === 0) {
      return;
    }

    fitAddon.fit();
    onResize?.(terminal.rows, terminal.cols);

    if (interactive && active) {
      terminal.focus();
    }
  }

  function scheduleFit() {
    if (fitScheduled) {
      return;
    }

    fitScheduled = true;
    requestAnimationFrame(() => {
      fitScheduled = false;
      fitTerminal();
    });
  }

  onMount(() => {
    let disposed = false;

    async function setup() {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import('@xterm/xterm'),
        import('@xterm/addon-fit')
      ]);
      if (disposed || !host) {
        return;
      }

      terminal = new Terminal({
        allowTransparency: false,
        convertEol: true,
        cursorBlink: interactive,
        disableStdin: !interactive,
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace',
        fontSize: 12,
        lineHeight: 1.45,
        scrollback: interactive ? 5000 : 20000
      });
      fitAddon = new FitAddon();
      terminal.loadAddon(fitAddon);
      terminal.open(host);
      applyTheme();
      syncTerminal(true);
      scheduleFit();

      if (interactive && onData) {
        dataListener = terminal.onData((data) => onData(data));
      }

      resizeObserver = new ResizeObserver(() => scheduleFit());
      resizeObserver.observe(host);

      themeObserver = new MutationObserver(() => applyTheme());
      themeObserver.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['class', 'style', 'data-accent']
      });
    }

    void setup();

    return () => {
      disposed = true;
      themeObserver?.disconnect();
      resizeObserver?.disconnect();
      dataListener?.dispose();
      terminal?.dispose();
      themeObserver = null;
      resizeObserver = null;
      dataListener = null;
      terminal = null;
      fitAddon = null;
      renderedText = '';
    };
  });

  $effect(() => {
    content;
    emptyText;
    syncTerminal();
  });

  $effect(() => {
    if (active) {
      scheduleFit();
    }
  });
</script>

<div class={`terminal-surface ${heightClass}`}>
  <div bind:this={host} class="h-full w-full bg-background px-3 py-2 text-foreground"></div>
</div>
