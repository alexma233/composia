import { writable, type Unsubscriber } from "svelte/store";

import { defaultLocale, setLocale, type Locale } from "$lib/i18n";

export const availableThemeModes = ["system", "light", "dark"] as const;
export const availableAccentColors = [
  "blue",
  "emerald",
  "violet",
  "rose",
  "amber",
] as const;

export type ThemeMode = (typeof availableThemeModes)[number];
export type AccentColor = (typeof availableAccentColors)[number];

const storageKeys = {
  locale: "composia.locale",
  themeMode: "composia.theme-mode",
  accentColor: "composia.accent-color",
} as const;

export const accentMetadata: Record<
  AccentColor,
  { label: string; labelZhHans: string; preview: string }
> = {
  blue: { label: "Blue", labelZhHans: "蓝色", preview: "hsl(221 83% 53%)" },
  emerald: { label: "Emerald", labelZhHans: "翠绿", preview: "hsl(160 84% 39%)" },
  violet: { label: "Violet", labelZhHans: "紫罗兰", preview: "hsl(262 83% 58%)" },
  rose: { label: "Rose", labelZhHans: "玫瑰", preview: "hsl(347 77% 50%)" },
  amber: { label: "Amber", labelZhHans: "琥珀", preview: "hsl(38 92% 50%)" },
};

function readStoredValue(key: string) {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage.getItem(key);
}

function normalizeThemeMode(value: string | null | undefined): ThemeMode {
  if (value === "light" || value === "dark" || value === "system") {
    return value;
  }
  return "system";
}

function normalizeAccentColor(value: string | null | undefined): AccentColor {
  if (value && availableAccentColors.includes(value as AccentColor)) {
    return value as AccentColor;
  }
  return "blue";
}

function normalizeLocale(value: string | null | undefined): Locale {
  if (value === "en-US" || value === "zh-Hans") {
    return value;
  }
  return defaultLocale;
}

const initialThemeMode = normalizeThemeMode(
  typeof document === "undefined"
    ? "system"
    : document.documentElement.dataset.themeMode,
);
const initialAccentColor = normalizeAccentColor(
  typeof document === "undefined"
    ? "blue"
    : document.documentElement.dataset.accent,
);
const initialLocale = normalizeLocale(
  typeof document === "undefined"
    ? defaultLocale
    : document.documentElement.lang,
);

export const themeMode = writable<ThemeMode>(initialThemeMode);
export const accentColor = writable<AccentColor>(initialAccentColor);
export const preferredLocale = writable<Locale>(initialLocale);

function resolveThemeMode(mode: ThemeMode) {
  if (mode === "system" && typeof window !== "undefined") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }
  return mode;
}

function applyThemeMode(mode: ThemeMode) {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  const resolved = resolveThemeMode(mode);
  root.dataset.themeMode = mode;
  root.classList.toggle("dark", resolved === "dark");
  root.style.colorScheme = resolved;
  window.localStorage.setItem(storageKeys.themeMode, mode);
}

function applyAccentColor(accent: AccentColor) {
  if (typeof document === "undefined") {
    return;
  }
  document.documentElement.dataset.accent = accent;
  window.localStorage.setItem(storageKeys.accentColor, accent);
}

function applyLocale(locale: Locale) {
  if (typeof document === "undefined") {
    return;
  }
  document.documentElement.lang = locale;
  window.localStorage.setItem(storageKeys.locale, locale);
  setLocale(locale);
}

export function setThemeMode(nextMode: ThemeMode) {
  themeMode.set(nextMode);
}

export function setAccentColor(nextAccent: AccentColor) {
  accentColor.set(nextAccent);
}

export function setPreferredLocale(nextLocale: Locale) {
  preferredLocale.set(nextLocale);
}

export function initializePreferences() {
  const storedThemeMode = normalizeThemeMode(
    readStoredValue(storageKeys.themeMode),
  );
  const storedAccentColor = normalizeAccentColor(
    readStoredValue(storageKeys.accentColor),
  );
  const storedLocale = normalizeLocale(readStoredValue(storageKeys.locale));

  themeMode.set(storedThemeMode);
  accentColor.set(storedAccentColor);
  preferredLocale.set(storedLocale);

  const unsubscribers: Unsubscriber[] = [
    themeMode.subscribe((mode) => applyThemeMode(mode)),
    accentColor.subscribe((accent) => applyAccentColor(accent)),
    preferredLocale.subscribe((locale) => applyLocale(locale)),
  ];

  const media =
    typeof window === "undefined"
      ? null
      : window.matchMedia("(prefers-color-scheme: dark)");
  const handleSystemThemeChange = () => {
    let currentMode: ThemeMode = "system";
    const unsubscribe = themeMode.subscribe((mode) => {
      currentMode = mode;
    });
    unsubscribe();
    if (currentMode === "system") {
      applyThemeMode(currentMode);
    }
  };

  media?.addEventListener("change", handleSystemThemeChange);

  return () => {
    unsubscribers.forEach((unsubscribe) => unsubscribe());
    media?.removeEventListener("change", handleSystemThemeChange);
  };
}
