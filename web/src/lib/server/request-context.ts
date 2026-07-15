import { AsyncLocalStorage } from "node:async_hooks";

const requestSignalStorage = new AsyncLocalStorage<AbortSignal>();

export function withRequestSignal<T>(
  signal: AbortSignal,
  callback: () => T,
): T {
  return requestSignalStorage.run(signal, callback);
}

export function currentRequestSignal() {
  return requestSignalStorage.getStore();
}
