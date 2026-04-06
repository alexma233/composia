import type { Snippet } from "svelte";

export type TabsContext = {
  value: string;
  setValue: (value: string) => void;
};

export const tabsContextKey = Symbol("tabs");

export type TabsTriggerProps = {
  value?: string;
  class?: string;
  children?: Snippet;
  [key: string]: unknown;
};

export type TabsContentProps = {
  value?: string;
  class?: string;
  children?: Snippet;
  [key: string]: unknown;
};
