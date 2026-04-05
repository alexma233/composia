import type { Writable } from 'svelte/store';

export type TabsContext = {
  value: Writable<string>;
};

export const tabsContextKey = Symbol('tabs');
