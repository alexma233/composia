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

export function normalizeLocale(value: string | null | undefined): Locale {
  if (!value) {
    return defaultLocale;
  }

  return (
    availableLocales.find(
      (locale) => locale.toLowerCase() === value.toLowerCase(),
    ) ?? defaultLocale
  );
}
