import type { UsagePoint } from "../types/admin";

export function formatDate(value: string): string {
  if (!value || value.startsWith("0001-")) return "-";
  return new Date(value).toLocaleString();
}

export function isoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function isoMinute(date: Date): string {
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${isoDate(date)}T${hours}:${minutes}`;
}

export function displayMinute(value: string): string {
  return value.replace("T", " ").replace(/-/g, "/");
}

export function addMinutes(date: Date, minutes: number): Date {
  const next = new Date(date);
  next.setMinutes(next.getMinutes() + minutes);
  return next;
}

export function floorToMinuteBucket(date: Date, bucketMinutes: number): Date {
  const next = new Date(date);
  const bucket = Math.max(1, bucketMinutes);
  next.setSeconds(0, 0);
  next.setMinutes(Math.floor(next.getMinutes() / bucket) * bucket);
  return next;
}

export function naturalDayRange(date = new Date()): { from: string; to: string } {
  const from = new Date(date);
  from.setHours(0, 0, 0, 0);
  const to = new Date(from);
  to.setDate(to.getDate() + 1);
  return { from: isoMinute(from), to: isoMinute(to) };
}

export function naturalWeekRange(date = new Date()): { from: string; to: string } {
  const from = new Date(date);
  from.setHours(0, 0, 0, 0);
  const day = from.getDay();
  const mondayOffset = day === 0 ? -6 : 1 - day;
  from.setDate(from.getDate() + mondayOffset);
  const to = new Date(from);
  to.setDate(to.getDate() + 7);
  return { from: isoMinute(from), to: isoMinute(to) };
}

export function naturalMonthRange(date = new Date()): { from: string; to: string } {
  const from = new Date(date.getFullYear(), date.getMonth(), 1);
  const to = new Date(date.getFullYear(), date.getMonth() + 1, 1);
  return { from: isoMinute(from), to: isoMinute(to) };
}

export function usagePath(filter: { key_id: string; model: string; from: string; to: string }) {
  const params = new URLSearchParams({ from: filter.from, to: filter.to });
  if (filter.key_id !== "all") params.set("key_id", filter.key_id);
  if (filter.model !== "all") params.set("model", filter.model);
  return `/admin/api/usage?${params.toString()}`;
}

export function fillUsageSeries(series: UsagePoint[], from: string, to: string, bucketMinutes = 1): UsagePoint[] {
  const start = floorToMinuteBucket(new Date(from), bucketMinutes);
  const end = new Date(to);
  const step = Math.max(1, bucketMinutes);
  const minuteCount = Math.floor((end.getTime() - start.getTime()) / 60000);
  if (!Number.isFinite(minuteCount) || minuteCount < 0) return series;
  if (Math.floor(minuteCount / step) > 1440) {
    return series.length ? series : [
      { date: from, requests: 0, errors: 0, avg_latency_ms: 0 },
      { date: to, requests: 0, errors: 0, avg_latency_ms: 0 },
    ];
  }
  const byDate = new Map(series.map((point) => [point.date, point]));
  const out: UsagePoint[] = [];
  let cursor = start;
  while (cursor < end) {
    const date = isoMinute(cursor);
    out.push(byDate.get(date) ?? { date, requests: 0, errors: 0, avg_latency_ms: 0 });
    cursor = addMinutes(cursor, step);
  }
  return out;
}
