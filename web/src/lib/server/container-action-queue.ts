const containerActionQueues = new Map<string, Promise<unknown>>();

export async function serializeContainerAction<T>(
  key: string,
  action: () => Promise<T>,
): Promise<T> {
  const previous = containerActionQueues.get(key) ?? Promise.resolve();
  const next = previous.catch(() => undefined).then(action);
  containerActionQueues.set(key, next);

  try {
    return await next;
  } finally {
    if (containerActionQueues.get(key) === next) {
      containerActionQueues.delete(key);
    }
  }
}
