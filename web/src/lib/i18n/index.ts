import { derived, writable } from "svelte/store";

import { de } from "$lib/i18n/messages/de";
import { enUS, type Dictionary } from "$lib/i18n/messages/en-us";
import { fr } from "$lib/i18n/messages/fr";
import { ja } from "$lib/i18n/messages/ja";
import { zhHans } from "$lib/i18n/messages/zh-hans";
import { zhHant } from "$lib/i18n/messages/zh-hant";

export const availableLocales = [
  "en-US",
  "zh-Hans",
  "zh-Hant",
  "ja",
  "de",
  "fr",
] as const;

export type Locale = (typeof availableLocales)[number];

export const defaultLocale: Locale = "en-US";

export type { Dictionary };

const dictionaries: Record<Locale, any> = {
  "en-US": enUS,
  "zh-Hans": zhHans,
  "zh-Hant": zhHant,
  ja: ja,
  de: de,
  fr: fr,
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
