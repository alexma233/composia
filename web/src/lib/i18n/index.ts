import { getContext, setContext } from "svelte";
import { writable } from "svelte/store";

import type { Dictionary } from "$lib/i18n/messages/en-us";
import {
  availableLocales,
  defaultLocale,
  normalizeLocale,
  type Locale,
} from "$lib/i18n/locales";

export { availableLocales, defaultLocale, normalizeLocale, type Locale };
export type { Dictionary };

const dictionaryLoaders: Record<Locale, () => Promise<Dictionary>> = {
  "en-US": () => import("$lib/i18n/messages/en-us").then(({ enUS }) => enUS),
  "zh-Hans": () =>
    import("$lib/i18n/messages/zh-hans").then(({ zhHans }) => zhHans),
  "zh-Hant": () =>
    import("$lib/i18n/messages/zh-hant").then(({ zhHant }) => zhHant),
  ja: () => import("$lib/i18n/messages/ja").then(({ ja }) => ja),
  de: () => import("$lib/i18n/messages/de").then(({ de }) => de),
  fr: () => import("$lib/i18n/messages/fr").then(({ fr }) => fr),
};

const contextKey = Symbol("composia-i18n");

type I18nContext = {
  messages: ReturnType<typeof writable<Dictionary>>;
  setLocale: (locale: string) => Promise<void>;
};

export async function loadDictionary(locale: Locale) {
  return dictionaryLoaders[locale]();
}

export function createI18nContext(locale: Locale, dictionary: Dictionary) {
  const messages = writable(dictionary);
  let requestId = 0;

  const context: I18nContext = {
    messages,
    async setLocale(value) {
      const nextLocale = normalizeLocale(value);
      const currentRequest = ++requestId;
      const nextDictionary = await loadDictionary(nextLocale);
      if (currentRequest === requestId) {
        messages.set(nextDictionary);
      }
    },
  };

  setContext(contextKey, context);
  return context;
}

export function getMessages() {
  const context = getContext<I18nContext>(contextKey);
  if (!context) {
    throw new Error("The i18n context is not initialized.");
  }
  return context.messages;
}
