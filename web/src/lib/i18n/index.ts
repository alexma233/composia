import { derived, writable } from 'svelte/store';

import { enUS, type Dictionary } from '$lib/i18n/messages/en-us';

export const availableLocales = ['en-US'] as const;

export type Locale = (typeof availableLocales)[number];

export const defaultLocale: Locale = 'en-US';

const dictionaries: Record<Locale, Dictionary> = {
  'en-US': enUS
};

function normalizeLocale(value: string | null | undefined): Locale {
  if (!value) {
    return defaultLocale;
  }
  const match = availableLocales.find((locale) => locale.toLowerCase() === value.toLowerCase());
  return match ?? defaultLocale;
}

const initialLocale = normalizeLocale(typeof document === 'undefined' ? defaultLocale : document.documentElement.lang);

export const locale = writable<Locale>(initialLocale);

export const messages = derived(locale, ($locale) => dictionaries[$locale] ?? dictionaries[defaultLocale]);

export function setLocale(nextLocale: string) {
  locale.set(normalizeLocale(nextLocale));
}
