type PollingOptions = {
  intervalMs: number;
  errorIntervalMs?: number;
  initialDelayMs?: number;
  runImmediately?: boolean;
};

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

    try {
      const shouldContinue = await tick();
      if (shouldContinue === false) {
        stop();
        return;
      }

      schedule(options.intervalMs);
    } catch {
      schedule(options.errorIntervalMs ?? options.intervalMs);
    }
  };

  if (options.runImmediately) {
    void run();
  } else {
    schedule(options.initialDelayMs ?? options.intervalMs);
  }

  return stop;
}
