import { derived, writable } from "svelte/store";

import { enUS, type Dictionary } from "$lib/i18n/messages/en-us";
import { zhHans } from "$lib/i18n/messages/zh-hans";

export const availableLocales = ["en-US", "zh-Hans"] as const;

export type Locale = (typeof availableLocales)[number];

export const defaultLocale: Locale = "en-US";

export type { Dictionary };

const dictionaries: Record<Locale, any> = {
  "en-US": enUS,
  "zh-Hans": zhHans,
};

function normalizeLocale(value: string | null | undefined): Locale {
  if (!value) {
    return defaultLocale;
  }
  const match = availableLocales.find(
    (locale) => locale.toLowerCase() === value.toLowerCase(),
  );
  return match ?? defaultLocale;
}

const initialLocale = normalizeLocale(
  typeof document === "undefined"
    ? defaultLocale
    : document.documentElement.lang,
);

export const locale = writable<Locale>(initialLocale);

export const messages = derived<typeof locale, Dictionary>(
  locale,
  ($locale) => dictionaries[$locale] ?? dictionaries[defaultLocale],
);

export function setLocale(nextLocale: string) {
  locale.set(normalizeLocale(nextLocale));
}
