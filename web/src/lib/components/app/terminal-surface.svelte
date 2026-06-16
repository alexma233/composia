<script lang="ts">
  import { onMount } from 'svelte';

  import { messages } from '$lib/i18n';
  import { observeThemeChange } from '$lib/theme-observer';

  type DataHandler = (data: string) => void;
  type ResizeHandler = (rows: number, cols: number) => void;

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

  const TERMINAL_RESET = '\x1bc\x1b[3J\x1b[H\x1b[2J';

  let terminal: import('@wterm/dom').WTerm | null = null;
  let disconnectThemeObserver: (() => void) | null = null;
  let scrollScheduled = false;
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

  function applyTheme(isDark?: boolean) {
    if (!terminal) {
      return;
    }

    const dark = isDark ?? document.documentElement.classList.contains('dark');
    terminal.element.classList.toggle('theme-composia-dark', dark);
    terminal.element.classList.toggle('theme-composia-light', !dark);
  }

  function disableReadOnlyInput() {
    if (interactive || !host) {
      return;
    }

    const input = host.querySelector('textarea');
    input?.setAttribute('tabindex', '-1');
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
      const [{ WTerm }, { GhosttyCore }] = await Promise.all([
        import('@wterm/dom'),
        import('@wterm/ghostty')
      ]);
      if (disposed || !host) {
        return;
      }

      const core = await GhosttyCore.load({
        scrollbackLimit: interactive ? 5000 : 20000
      });
      if (disposed || !host) {
        return;
      }

      terminal = new WTerm(host, {
        autoResize: true,
        core,
        cursorBlink: interactive,
        onData: interactive ? (data) => onData?.(data) : () => {},
        onResize: (cols, rows) => onResize?.(rows, cols)
      });

      host.addEventListener('click', stopReadOnlyFocus, { capture: true });
      await terminal.init();
      if (disposed) {
        terminal.destroy();
        terminal = null;
        return;
      }

      applyTheme();
      disableReadOnlyInput();
      syncTerminal(true);

      disconnectThemeObserver = observeThemeChange((isDark) => applyTheme(isDark));
    }

    void setup();

    return () => {
      disposed = true;
      disconnectThemeObserver?.();
      host?.removeEventListener('click', stopReadOnlyFocus, { capture: true });
      terminal?.destroy();
      disconnectThemeObserver = null;
      terminal = null;
      scrollScheduled = false;
      renderedText = '';
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

<div class={`terminal-surface ${heightClass}`}>
  <div bind:this={host} class="h-full w-full" aria-label={$messages.common.terminal}></div>
</div>
