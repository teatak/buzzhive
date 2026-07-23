import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { localeStore, type Locale } from "../i18n/locale"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatCompactNumber(value: number) {
  return formatCompactNumberForLocale(value, localeStore.getSnapshot());
}

export function formatCompactNumberForLocale(value: number, locale: Locale) {
  if (!Number.isFinite(value)) return String(value)

  const abs = Math.abs(value);
  const units = compactUnits(locale);
  const unit = units.find((item) => abs >= item.value);
  if (!unit) return String(value);

  const scaled = value / unit.value;
  const digits = Math.abs(scaled) >= 100 ? 0 : 1;
  return `${scaled.toFixed(digits).replace(/\.0$/, "")}${unit.suffix}`;
}

function compactUnits(locale: Locale) {
  if (locale === "zh-CN") {
    return [
      { value: 100_000_000, suffix: "亿" },
      { value: 10_000, suffix: "万" },
    ];
  }
  if (locale === "zh-TW") {
    return [
      { value: 100_000_000, suffix: "億" },
      { value: 10_000, suffix: "萬" },
    ];
  }
  return [
    { value: 1_000_000_000, suffix: "B" },
    { value: 1_000_000, suffix: "M" },
    { value: 1_000, suffix: "K" },
  ];
}
