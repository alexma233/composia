type PollingOptions = {
  intervalMs: number | (() => number);
  errorIntervalMs?: number;
  initialDelayMs?: number;
  runImmediately?: boolean;
};

const activeStatuses = new Set(["pending", "running", "awaiting_confirmation"]);

export function hasActiveOperations(statuses: Iterable<string>) {
  return Array.from(statuses).some((status) => activeStatuses.has(status));
}

export function startPolling(
  tick: () => void | boolean | Promise<void | boolean>,
  options: PollingOptions,
) {
  let cancelled = false;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const clear = () => {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  };

  const stop = () => {
    cancelled = true;
    clear();
  };

  const interval = () =>
    typeof options.intervalMs === "function"
      ? options.intervalMs()
      : options.intervalMs;

  const schedule = (delay: number) => {
    if (cancelled) {
      return;
    }

    clear();
    timer = setTimeout(run, delay);
  };

  const run = async () => {
    if (cancelled) {
      return;
    }

    if (typeof document !== "undefined" && document.hidden) {
      schedule(interval());
      return;
    }

    try {
      const shouldContinue = await tick();
      if (shouldContinue === false) {
        stop();
        return;
      }

      schedule(interval());
    } catch {
      schedule(options.errorIntervalMs ?? interval());
    }
  };

  if (options.runImmediately) {
    void run();
  } else {
    schedule(options.initialDelayMs ?? interval());
  }

  return stop;
}
