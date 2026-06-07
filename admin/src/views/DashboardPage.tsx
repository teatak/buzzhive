import { Activity, BarChart3, CircleOff, KeyRound } from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { ButtonGroup } from "../components/ui/button-group";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/select";
import { TokenUsageChart } from "../features/usage/TokenUsageChart";
import { UsageChart } from "../features/usage/UsageChart";
import { useLocale } from "../i18n/locale";
import { displayMinute, naturalMonthRange } from "../lib/date";
import { cn } from "../lib/utils";
import type { Model, UsagePoint, UsageSummary, UserAPIKey } from "../types/admin";

type UsageFilter = {
  key_id: string;
  model: string;
  from: string;
  to: string;
};

export function DashboardPage(props: {
  usage: UsageSummary | null;
  usageFilter: UsageFilter;
  usageIsToday: boolean;
  usageSeries: UsagePoint[];
  tokenUsage: UsageSummary | null;
  tokenUsageFilter: UsageFilter;
  tokenUsageIsToday: boolean;
  tokenUsageSeries: UsagePoint[];
  userAPIKeys: UserAPIKey[];
  models: Model[];
  ownActiveKeys: UserAPIKey[];
  onUsageFilterChange: (filter: UsageFilter) => void;
  onResetUsageToToday: () => void;
  onSelectUsageRange: (from: string, to: string) => void;
  onTokenUsageFilterChange: (filter: UsageFilter) => void;
  onResetTokenUsageToToday: () => void;
}) {
  const { t } = useLocale();

  return (
    <div className="stack">
      <section className="metrics">
        <Card>
          <CardContent className="metric-content">
            <div className="metric-label"><Activity size={17} /> {t("dashboard.requests")}</div>
            <div className="metric-value">{props.usage?.requests ?? 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="metric-content">
            <div className="metric-label"><KeyRound size={17} /> {t("nav.my_keys")}</div>
            <div className="metric-value">{props.ownActiveKeys.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="metric-content">
            <div className="metric-label"><CircleOff size={17} /> {t("dashboard.errors")}</div>
            <div className="metric-value">{props.usage?.errors ?? 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="metric-content">
            <div className="metric-label"><BarChart3 size={17} /> {t("dashboard.avg_latency")}</div>
            <div className="metric-value">{Math.round(props.usage?.avg_latency_ms ?? 0)}ms</div>
          </CardContent>
        </Card>
      </section>

      <section className="dashboard-usage-grid">
        <div className="stack">
          <Card>
            <CardHeader>
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="shrink-0">{t("dashboard.requests")}</CardTitle>
                <Badge variant="outline" className="shrink-0">
                  {usageBucketLabel(props.usage?.bucket_minutes ?? 1, t)}
                </Badge>
                <Badge variant="secondary" className="min-w-0 truncate">
                  {displayMinute(props.usageFilter.from)} - {displayMinute(props.usageFilter.to)}
                </Badge>
              </div>
              <UsageRangeAction
                filter={props.usageFilter}
                isToday={props.usageIsToday}
                onChange={props.onUsageFilterChange}
                onResetToday={props.onResetUsageToToday}
              />
            </CardHeader>
            <CardContent>
              <UsageFilterControls
                filter={props.usageFilter}
                userAPIKeys={props.userAPIKeys}
                models={props.models}
                onChange={props.onUsageFilterChange}
              />
              <UsageChart series={props.usageSeries} bucketMinutes={props.usage?.bucket_minutes ?? 1} onRangeSelect={props.onSelectUsageRange} />
            </CardContent>
          </Card>

          <TokenUsageMetrics usage={props.tokenUsage} />

          <Card>
            <CardHeader>
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="shrink-0">{t("usage.tokens_title")}</CardTitle>
                <Badge variant="outline" className="shrink-0">
                  {usageBucketLabel(props.tokenUsage?.bucket_minutes ?? 1, t)}
                </Badge>
                <Badge variant="secondary" className="min-w-0 truncate">
                  {displayMinute(props.tokenUsageFilter.from)} - {displayMinute(props.tokenUsageFilter.to)}
                </Badge>
              </div>
              <UsageRangeAction
                filter={props.tokenUsageFilter}
                isToday={props.tokenUsageIsToday}
                onChange={props.onTokenUsageFilterChange}
                onResetToday={props.onResetTokenUsageToToday}
              />
            </CardHeader>
            <CardContent>
              <UsageFilterControls
                filter={props.tokenUsageFilter}
                userAPIKeys={props.userAPIKeys}
                models={props.models}
                onChange={props.onTokenUsageFilterChange}
              />
              <TokenUsageChart series={props.tokenUsageSeries} />
            </CardContent>
          </Card>
        </div>
      </section>
    </div>
  );
}

function RangeShortcut(props: { label: string; active: boolean; onClick: () => void }) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className={cn(props.active && "bg-muted text-foreground hover:bg-muted dark:bg-input/50 dark:hover:bg-input/50")}
      onClick={props.onClick}
    >
      {props.label}
    </Button>
  );
}

function UsageRangeAction(props: { filter: UsageFilter; isToday: boolean; onChange: (filter: UsageFilter) => void; onResetToday: () => void }) {
  const { t } = useLocale();

  return (
    <CardAction className="row-span-1 self-center">
      <ButtonGroup aria-label={t("usage.range_shortcuts")}>
        <RangeShortcut label={t("common.today")} active={props.isToday} onClick={props.onResetToday} />
        <RangeShortcut label={t("common.this_month")} active={isRange(props.filter, naturalMonthRange())} onClick={() => props.onChange({ ...props.filter, ...naturalMonthRange() })} />
      </ButtonGroup>
    </CardAction>
  );
}

function UsageFilterControls(props: { filter: UsageFilter; userAPIKeys: UserAPIKey[]; models: Model[]; onChange: (filter: UsageFilter) => void }) {
  const { t } = useLocale();

  return (
    <div className="usage-filters">
      <Select
        value={props.filter.key_id}
        onValueChange={(value) => props.onChange({ ...props.filter, key_id: value })}
      >
        <SelectTrigger size="sm" className="w-full rounded-md px-2 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("usage.all_my_keys")}</SelectItem>
          {props.userAPIKeys.map((key) => (
            <SelectItem key={key.id} value={String(key.id)}>{key.name}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={props.filter.model}
        onValueChange={(value) => props.onChange({ ...props.filter, model: value })}
      >
        <SelectTrigger size="sm" className="w-full rounded-md px-2 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("usage.all_models")}</SelectItem>
          {props.models.map((model) => (
            <SelectItem key={model.id} value={model.name}>{model.display_name || model.name}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        className="h-7 rounded-md px-2 text-xs md:text-xs"
        style={{ minWidth: 0 }}
        type="datetime-local"
        value={props.filter.from}
        onChange={(event) => props.onChange({ ...props.filter, from: event.target.value })}
      />
      <Input
        className="h-7 rounded-md px-2 text-xs md:text-xs"
        style={{ minWidth: 0 }}
        type="datetime-local"
        value={props.filter.to}
        onChange={(event) => props.onChange({ ...props.filter, to: event.target.value })}
      />
    </div>
  );
}

function TokenUsageMetrics(props: { usage: UsageSummary | null }) {
	const { t } = useLocale();
	const usage = props.usage;
	const promptTokens = usage?.prompt_tokens ?? 0;
	const cachedTokens = usage?.cached_tokens ?? 0;
	const items = [
		{ label: t("usage.total_tokens"), value: usage?.total_tokens ?? 0 },
		{ label: t("usage.input_cached_tokens_metric"), value: cachedTokens },
		{ label: t("usage.input_uncached_tokens_metric"), value: Math.max(0, promptTokens - cachedTokens) },
		{ label: t("usage.output_tokens"), value: usage?.completion_tokens ?? 0 },
	];

  return (
    <section className="metrics">
      {items.map((item) => (
        <Card key={item.label}>
          <CardContent className="metric-content">
            <div className="metric-label"><BarChart3 size={17} /> {item.label}</div>
            <div className="metric-value" title={item.value.toLocaleString()}>{formatCompactNumber(item.value)}</div>
          </CardContent>
        </Card>
      ))}
    </section>
  );
}

function formatCompactNumber(value: number) {
  const abs = Math.abs(value);
  const units = [
    { value: 1_000_000_000, suffix: "B" },
    { value: 1_000_000, suffix: "M" },
    { value: 1_000, suffix: "K" },
  ];
  const unit = units.find((item) => abs >= item.value);
  if (!unit) return value.toLocaleString();
  const scaled = value / unit.value;
  const digits = Math.abs(scaled) >= 100 ? 0 : 1;
  return `${scaled.toFixed(digits).replace(/\.0$/, "")}${unit.suffix}`;
}

function isRange(filter: UsageFilter, range: { from: string; to: string }) {
  return filter.from === range.from && filter.to === range.to;
}

function usageBucketLabel(bucketMinutes: number, t: (key: string, params?: Record<string, string | number>) => string) {
  if (bucketMinutes >= 1440) return t("usage.bucket_daily");
  if (bucketMinutes >= 60) return t("usage.bucket_hourly");
  return t("usage.bucket_detail", { minutes: bucketMinutes });
}
