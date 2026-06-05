import { Activity, BarChart3, CircleOff, KeyRound } from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/select";
import { UsageByKeyTable } from "../features/usage/UsageByKeyTable";
import { UsageChart } from "../features/usage/UsageChart";
import { useLocale } from "../i18n/locale";
import { displayMinute } from "../lib/date";
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
  userAPIKeys: UserAPIKey[];
  models: Model[];
  ownActiveKeys: UserAPIKey[];
  onUsageFilterChange: (filter: UsageFilter) => void;
  onResetUsageToToday: () => void;
  onSelectUsageRange: (from: string, to: string) => void;
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
        <Card>
          <CardHeader>
            <div className="flex min-w-0 items-center gap-2">
              <CardTitle className="shrink-0">{t("usage.title")}</CardTitle>
              <Badge variant="secondary" className="min-w-0 truncate">
                {displayMinute(props.usageFilter.from)} - {displayMinute(props.usageFilter.to)}
              </Badge>
            </div>
            {!props.usageIsToday && (
              <CardAction className="row-span-1 self-center">
                <Badge asChild variant="outline" className="cursor-pointer">
                  <button type="button" onClick={props.onResetUsageToToday}>{t("common.today")}</button>
                </Badge>
              </CardAction>
            )}
          </CardHeader>
          <CardContent>
            <div className="usage-filters">
              <Select
                value={props.usageFilter.key_id}
                onValueChange={(value) => props.onUsageFilterChange({ ...props.usageFilter, key_id: value })}
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
                value={props.usageFilter.model}
                onValueChange={(value) => props.onUsageFilterChange({ ...props.usageFilter, model: value })}
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
                value={props.usageFilter.from}
                onChange={(event) => props.onUsageFilterChange({ ...props.usageFilter, from: event.target.value })}
              />
              <Input
                className="h-7 rounded-md px-2 text-xs md:text-xs"
                style={{ minWidth: 0 }}
                type="datetime-local"
                value={props.usageFilter.to}
                onChange={(event) => props.onUsageFilterChange({ ...props.usageFilter, to: event.target.value })}
              />
            </div>
            <UsageChart series={props.usageSeries} bucketMinutes={props.usage?.bucket_minutes ?? 1} onRangeSelect={props.onSelectUsageRange} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle>{t("usage.by_key")}</CardTitle>
              <Badge variant="secondary">{Object.keys(props.usage?.by_key ?? {}).length} {t("accounts.keys")}</Badge>
            </div>
          </CardHeader>
          <CardContent>
            <UsageByKeyTable usage={props.usage?.by_key ?? {}} />
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
