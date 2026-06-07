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

export function currentTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

function toAPIInstant(value: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return date.toISOString();
}

function toAPILocalInstant(value: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	const offsetMinutes = -date.getTimezoneOffset();
	const sign = offsetMinutes >= 0 ? "+" : "-";
	const absOffset = Math.abs(offsetMinutes);
	const offsetHours = String(Math.floor(absOffset / 60)).padStart(2, "0");
	const offsetRemainder = String(absOffset % 60).padStart(2, "0");
	return `${isoDate(date)}T${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}:${String(date.getSeconds()).padStart(2, "0")}${sign}${offsetHours}:${offsetRemainder}`;
}

export function addMinutes(date: Date, minutes: number): Date {
  const next = new Date(date);
  next.setMinutes(next.getMinutes() + minutes);
  return next;
}

export function floorToMinuteBucket(date: Date, bucketMinutes: number): Date {
  const next = new Date(date);
  const bucket = Math.max(1, bucketMinutes);
  if (bucket >= 1440) {
    next.setHours(0, 0, 0, 0);
    return next;
  }
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

export function naturalMonthRange(date = new Date()): { from: string; to: string } {
  const from = new Date(date.getFullYear(), date.getMonth(), 1);
  const to = new Date(date.getFullYear(), date.getMonth() + 1, 1);
  return { from: isoMinute(from), to: isoMinute(to) };
}

export function usagePath(filter: { key_id: string; model: string; from: string; to: string }) {
	const params = new URLSearchParams({
		from: toAPILocalInstant(filter.from),
		to: toAPILocalInstant(filter.to),
		tz: currentTimeZone(),
	});
  if (filter.key_id !== "all") params.set("key_id", filter.key_id);
  if (filter.model !== "all") params.set("model", filter.model);
  return `/admin/api/usage?${params.toString()}`;
}

function usageDateKey(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toISOString();
}

function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

function localDateLabel(date: Date): string {
  return `${date.getFullYear()}/${pad2(date.getMonth() + 1)}/${pad2(date.getDate())}`;
}

export function formatUsageLabel(value: string, bucketMinutes = 1): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return displayMinute(value);
  if (bucketMinutes >= 1440) {
    return localDateLabel(date);
  }
  return `${localDateLabel(date)} ${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

export function usageTooltip(value: string, bucketMinutes = 1): string {
  return formatUsageLabel(value, bucketMinutes);
}

function emptyUsagePoint(date: string, bucketMinutes: number): UsagePoint {
  return {
    date,
    label: formatUsageLabel(date, bucketMinutes),
    tooltip: usageTooltip(date, bucketMinutes),
    requests: 0,
    errors: 0,
    avg_latency_ms: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    total_tokens: 0,
    cached_tokens: 0,
    reasoning_tokens: 0,
  };
}

export function fillUsageSeries(series: UsagePoint[], from: string, to: string, bucketMinutes = 1): UsagePoint[] {
  const start = floorToMinuteBucket(new Date(from), bucketMinutes);
  const end = new Date(to);
  const step = Math.max(1, bucketMinutes);
  const minuteCount = Math.floor((end.getTime() - start.getTime()) / 60000);
  if (!Number.isFinite(minuteCount) || minuteCount < 0) return series;
  if (Math.floor(minuteCount / step) > 1440) {
    return series.length ? series : [
      emptyUsagePoint(toAPIInstant(from), bucketMinutes),
      emptyUsagePoint(toAPIInstant(to), bucketMinutes),
    ];
  }
  const byDate = new Map(series.map((point) => [usageDateKey(point.date), point]));
  const out: UsagePoint[] = [];
  let cursor = start;
  while (cursor < end) {
    const date = cursor.toISOString();
    out.push(byDate.get(usageDateKey(date)) ?? emptyUsagePoint(date, bucketMinutes));
    cursor = addMinutes(cursor, step);
  }
  return out;
}
