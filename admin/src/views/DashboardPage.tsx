import type { ReactNode } from "react";
import { Activity, BarChart3, CalendarClock, CircleGauge, CircleOff, KeyRound, Loader2 } from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { ButtonGroup } from "../components/ui/button-group";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Progress } from "../components/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/select";
import { TokenUsageChart } from "../features/usage/TokenUsageChart";
import { UsageChart } from "../features/usage/UsageChart";
import { useLocale } from "../i18n/locale";
import { displayMinute, formatDate, naturalMonthRange, recentDaysRange } from "../lib/date";
import { modelDisplayName } from "../lib/model";
import { cn, formatCompactNumber } from "../lib/utils";
import type { Model, UsagePoint, UsageSummary, UserAPIKey, UserQuotaStatus } from "../types/admin";

export type UsageFilter = {
  key_id: string;
  model: string;
  from: string;
  to: string;
};

export type UsageDashboardProps = {
  usage: UsageSummary | null;
  quota?: UserQuotaStatus | null;
  usageFilter: UsageFilter;
  usageIsToday: boolean;
  usageSeries: UsagePoint[];
  userAPIKeys: UserAPIKey[];
  models: Model[];
  ownActiveKeys: UserAPIKey[];
  onUsageFilterChange: (filter: UsageFilter) => void;
  onResetUsageToToday: () => void;
  onSelectUsageRange: (from: string, to: string) => void;
};

export function DashboardPage(props: UsageDashboardProps) {
  const { t } = useLocale();
  return (
    <UsageDashboard
      {...props}
      apiKeyMetricLabel={t("nav.my_keys")}
      allKeysLabel={t("usage.all_my_keys")}
    />
  );
}

export function UsageDashboard(props: UsageDashboardProps & { apiKeyMetricLabel: string; allKeysLabel: string }) {
  const { t } = useLocale();

  return (
    <div className="stack">
      {props.quota && <QuotaSummary quota={props.quota} />}

      <Card size="sm" className="overflow-visible">
        <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <CardTitle className="shrink-0">{t("usage.filters")}</CardTitle>
            {props.usage && (
              <Badge variant="outline" className="shrink-0">
                {usageBucketLabel(props.usage.bucket_minutes, t)}
              </Badge>
            )}
            <Badge variant="secondary" className="w-full min-w-0 truncate sm:w-auto">
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
            allKeysLabel={props.allKeysLabel}
            onChange={props.onUsageFilterChange}
          />
        </CardContent>
      </Card>

      {props.usage ? (
        <>
          <section className="metrics">
            <Card>
              <CardContent className="metric-content">
                <div className="metric-label"><Activity size={17} /> {t("dashboard.requests")}</div>
                <div className="metric-value">{props.usage.requests}</div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="metric-content">
                <div className="metric-label"><KeyRound size={17} /> {props.apiKeyMetricLabel}</div>
                <div className="metric-value">{props.ownActiveKeys.length}</div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="metric-content">
                <div className="metric-label"><CircleOff size={17} /> {t("dashboard.errors")}</div>
                <div className="metric-value">{props.usage.errors}</div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="metric-content">
                <div className="metric-label"><BarChart3 size={17} /> {t("dashboard.avg_latency")}</div>
                <div className="metric-value">{Math.round(props.usage.avg_latency_ms)}ms</div>
              </CardContent>
            </Card>
            <TokenUsageMetricCards usage={props.usage} />
          </section>

          <Card>
            <CardHeader>
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="shrink-0">{t("dashboard.requests")}</CardTitle>
                <Badge variant="outline" className="shrink-0">
                  {usageBucketLabel(props.usage.bucket_minutes, t)}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              <UsageChart series={props.usageSeries} bucketMinutes={props.usage.bucket_minutes} onRangeSelect={props.onSelectUsageRange} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="shrink-0">{t("usage.tokens_title")}</CardTitle>
                <Badge variant="outline" className="shrink-0">
                  {usageBucketLabel(props.usage.bucket_minutes, t)}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              <TokenUsageChart series={props.usageSeries} bucketMinutes={props.usage.bucket_minutes} onRangeSelect={props.onSelectUsageRange} />
            </CardContent>
          </Card>
        </>
      ) : (
        <DashboardLoadingPanel />
      )}
    </div>
  );
}

function QuotaSummary({ quota }: { quota: UserQuotaStatus }) {
  const { locale, t } = useLocale();
  if (quota.unlimited) {
    return (
      <Card size="sm">
        <CardContent className="flex items-center gap-2">
          <CircleGauge className="text-muted-foreground" size={17} />
          <span className="font-medium">{t("users.quota")}</span>
          <Badge variant="secondary">{t("users.quota_unlimited")}</Badge>
        </CardContent>
      </Card>
    );
  }

  return (
    <section aria-label={t("users.quota")} className="grid gap-4 md:grid-cols-2">
      <QuotaCard
        icon={<CalendarClock size={17} />}
        label={t("users.weekly_quota_remaining")}
        remaining={quota.weekly_remaining_microcredits}
        total={quota.weekly_quota_credits}
        locale={locale}
        emptyLabel={t("users.no_weekly_quota")}
        detail={quota.weekly_quota_credits > 0 ? (
          <span title={formatDate(quota.period_end)}>
            {t("users.quota_resets_at")} {formatDate(quota.period_end)}
          </span>
        ) : undefined}
      />
      <QuotaCard
        icon={<CircleGauge size={17} />}
        label={t("users.lifetime_quota_remaining")}
        remaining={quota.lifetime_remaining_microcredits}
        total={quota.lifetime_quota_credits}
        locale={locale}
      />
    </section>
  );
}

function QuotaCard(props: {
  icon: ReactNode;
  label: string;
  remaining: number;
  total: number;
  locale: string;
  emptyLabel?: string;
  detail?: ReactNode;
}) {
  const totalMicrocredits = props.total * 1_000_000;
  const progress = totalMicrocredits > 0
    ? Math.min(100, Math.max(0, (props.remaining / totalMicrocredits) * 100))
    : 0;
  const value = props.total > 0
    ? `${formatCredits(props.remaining, props.locale)} / ${formatCredits(totalMicrocredits, props.locale)}`
    : (props.emptyLabel ?? formatCredits(0, props.locale));

  return (
    <Card size="sm">
      <CardContent className="flex h-full flex-col gap-2.5">
        <div className="flex min-w-0 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2 text-sm font-medium text-muted-foreground">
            {props.icon}
            <span className="truncate">{props.label}</span>
          </div>
          <span className="shrink-0 text-sm font-semibold tabular-nums">{value}</span>
        </div>
        <Progress value={progress} />
        {props.detail && <div className="text-xs text-muted-foreground">{props.detail}</div>}
      </CardContent>
    </Card>
  );
}

function formatCredits(microcredits: number, locale: string) {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 3 }).format(microcredits / 1_000_000);
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
    <div className="self-start sm:self-center">
      <ButtonGroup aria-label={t("usage.range_shortcuts")}>
        <RangeShortcut label={t("common.today")} active={props.isToday} onClick={props.onResetToday} />
        <RangeShortcut label={t("common.last_3_days")} active={isRange(props.filter, recentDaysRange(3))} onClick={() => props.onChange({ ...props.filter, ...recentDaysRange(3) })} />
        <RangeShortcut label={t("common.this_month")} active={isRange(props.filter, naturalMonthRange())} onClick={() => props.onChange({ ...props.filter, ...naturalMonthRange() })} />
      </ButtonGroup>
    </div>
  );
}

function UsageFilterControls(props: { filter: UsageFilter; userAPIKeys: UserAPIKey[]; models: Model[]; allKeysLabel: string; onChange: (filter: UsageFilter) => void }) {
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
          <SelectItem value="all">{props.allKeysLabel}</SelectItem>
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
            <SelectItem key={model.id} value={model.name}>{modelDisplayName(model)}</SelectItem>
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

function TokenUsageMetricCards(props: { usage: UsageSummary }) {
	const { t } = useLocale();
	const usage = props.usage;
	const promptTokens = usage.prompt_tokens;
	const cachedTokens = usage.cached_tokens;
	const items = [
		{ label: t("usage.total_tokens"), value: usage.total_tokens },
		{ label: t("usage.input_cached_tokens_metric"), value: cachedTokens },
		{ label: t("usage.input_uncached_tokens_metric"), value: Math.max(0, promptTokens - cachedTokens) },
		{ label: t("usage.output_tokens"), value: usage.completion_tokens },
	];

  return (
    <>
      {items.map((item) => (
        <Card key={item.label}>
          <CardContent className="metric-content">
            <div className="metric-label"><BarChart3 size={17} /> {item.label}</div>
            <div className="metric-value" title={String(item.value)}>{formatCompactNumber(item.value)}</div>
          </CardContent>
        </Card>
      ))}
    </>
  );
}

function DashboardLoadingPanel() {
  return (
    <Card>
      <CardContent className="flex min-h-[27.6rem] items-center justify-center">
        <Loader2 className="size-7 animate-spin text-muted-foreground" />
      </CardContent>
    </Card>
  );
}

function isRange(filter: UsageFilter, range: { from: string; to: string }) {
  return filter.from === range.from && filter.to === range.to;
}

function usageBucketLabel(bucketMinutes: number, t: (key: string, params?: Record<string, string | number>) => string) {
  if (bucketMinutes >= 1440) return t("usage.bucket_daily");
  if (bucketMinutes >= 60) return t("usage.bucket_hourly");
  return t("usage.bucket_detail", { minutes: bucketMinutes });
}
