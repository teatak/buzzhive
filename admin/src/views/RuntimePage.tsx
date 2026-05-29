import { Alert, AlertDescription } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { useLocale } from "../i18n/locale";
import { formatDate } from "../lib/date";
import type { AdminConfig, AdminKey, ModelUsageSummary, Stats } from "../types/admin";

type ModelUsageFilter = {
  key_id: string;
  from: string;
  to: string;
};

function shortText(value: string, max = 96) {
  return value.length > max ? `${value.slice(0, max)}...` : value;
}

export function RuntimePage(props: {
  stats: Stats;
  config: AdminConfig;
  coolingKeys: [string, string][];
  keys: AdminKey[];
  modelUsageFilter: ModelUsageFilter;
  modelUsageTotals: ModelUsageSummary["total_by_model"];
  modelUsageSeries: ModelUsageSummary["series"];
  modelUsageAccountTotals: ModelUsageSummary["account_totals"];
  quotaSignals: ModelUsageSummary["quota_signals"];
  recentModelErrors: ModelUsageSummary["recent_errors"];
  onModelUsageFilterChange: (filter: ModelUsageFilter) => void;
}) {
  const { t } = useLocale();

  return (
    <section className="runtime-grid">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle>{t("nav.runtime")}</CardTitle>
            <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("runtime.online")}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableBody>
              <TableRow><TableCell className="muted">{t("runtime.started")}</TableCell><TableCell className="right mono">{formatDate(props.stats.started_at)}</TableCell></TableRow>
              <TableRow><TableCell className="muted">{t("runtime.last_request")}</TableCell><TableCell className="right mono">{formatDate(props.stats.last_updated)}</TableCell></TableRow>
              <TableRow><TableCell className="muted">{t("runtime.timeout")}</TableCell><TableCell className="right mono">{props.config.timeout}</TableCell></TableRow>
              <TableRow><TableCell className="muted">{t("runtime.cooldown")}</TableCell><TableCell className="right mono">{`${props.config.cooldown_seconds}s`}</TableCell></TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle>{t("runtime.cooling_keys")}</CardTitle>
            <Badge variant="outline" className="border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-300">{props.coolingKeys.length}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          {props.coolingKeys.length ? (
            <Table>
              <TableBody>
                {props.coolingKeys.map(([key, expires]) => (
                  <TableRow key={key}><TableCell className="mono">{key}</TableCell><TableCell className="right mono">{formatDate(expires)}</TableCell></TableRow>
                ))}
              </TableBody>
            </Table>
          ) : <Alert><AlertDescription>{t("runtime.no_cooling_keys")}</AlertDescription></Alert>}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle>{t("model.usage")}</CardTitle>
            <Badge variant="secondary">{props.modelUsageTotals.length} {t("model.model")}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <div className="filter-row">
            <Select
              value={props.modelUsageFilter.key_id}
              onValueChange={(value) => props.onModelUsageFilterChange({ ...props.modelUsageFilter, key_id: value })}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("usage.all_gemini_keys")}</SelectItem>
                {props.keys.map((key) => <SelectItem key={key.id} value={String(key.id)}>{key.name}</SelectItem>)}
              </SelectContent>
            </Select>
            <Input type="datetime-local" value={props.modelUsageFilter.from} onChange={(event) => props.onModelUsageFilterChange({ ...props.modelUsageFilter, from: event.target.value })} />
            <Input type="datetime-local" value={props.modelUsageFilter.to} onChange={(event) => props.onModelUsageFilterChange({ ...props.modelUsageFilter, to: event.target.value })} />
          </div>
          {props.modelUsageTotals.length ? (
            <div className="stack">
              <Table>
                <TableHeader>
                  <TableRow><TableHead>{t("model.model")}</TableHead><TableHead className="right">{t("model.calls")}</TableHead><TableHead className="right">{t("model.errors")}</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {props.modelUsageTotals.map((row) => (
                    <TableRow key={row.model}>
                      <TableCell className="mono">{row.model}</TableCell>
                      <TableCell className="right mono">{row.requests}</TableCell>
                      <TableCell className="right mono">{row.errors}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <Table>
                <TableHeader>
                  <TableRow><TableHead>{t("model.account")}</TableHead><TableHead>{t("model.model")}</TableHead><TableHead className="right">{t("model.calls")}</TableHead><TableHead className="right">429</TableHead><TableHead className="right">{t("model.keys")}</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {props.modelUsageAccountTotals.map((row) => (
                    <TableRow key={`${row.account_email}:${row.model}`}>
                      <TableCell>{row.account_email}</TableCell>
                      <TableCell className="mono">{row.model}</TableCell>
                      <TableCell className="right mono">{row.requests}</TableCell>
                      <TableCell className="right mono">{row.quota_429}</TableCell>
                      <TableCell className="right mono">{row.distinct_keys}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <Table>
                <TableHeader>
                  <TableRow><TableHead>{t("model.minute")}</TableHead><TableHead>{t("model.account")}</TableHead><TableHead>{t("model.model")}</TableHead><TableHead className="right">429</TableHead><TableHead className="right">{t("model.keys")}</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {props.quotaSignals.slice(0, 20).map((row) => (
                    <TableRow key={`${row.date}:${row.account_email}:${row.model}`}>
                      <TableCell className="mono">{row.date}</TableCell>
                      <TableCell>{row.account_email}</TableCell>
                      <TableCell className="mono">{row.model}</TableCell>
                      <TableCell className="right mono">{row.quota_429}</TableCell>
                      <TableCell className="right mono">{row.distinct_keys}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <Table>
                <TableHeader>
                  <TableRow><TableHead>{t("model.time")}</TableHead><TableHead>{t("model.key")}</TableHead><TableHead>{t("model.model")}</TableHead><TableHead className="right">{t("model.status")}</TableHead><TableHead>{t("model.error")}</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {props.recentModelErrors.map((row) => (
                    <TableRow key={`${row.date}:${row.key_name}:${row.attempt}`}>
                      <TableCell className="mono">{row.date}</TableCell>
                      <TableCell className="mono">{row.key_name}</TableCell>
                      <TableCell className="mono">{row.model}</TableCell>
                      <TableCell className="right mono">{row.status}</TableCell>
                      <TableCell title={row.error_body || row.error_message} className="mono">
                        {shortText(row.error_code || row.error_message || row.error_body || t("common.unknown"))}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <Table>
                <TableHeader>
                  <TableRow><TableHead>{t("model.minute")}</TableHead><TableHead>{t("model.model")}</TableHead><TableHead className="right">{t("model.calls")}</TableHead><TableHead className="right">{t("model.errors")}</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {props.modelUsageSeries.slice(-20).map((row) => (
                    <TableRow key={`${row.date}:${row.model}`}>
                      <TableCell className="mono">{row.date}</TableCell>
                      <TableCell className="mono">{row.model}</TableCell>
                      <TableCell className="right mono">{row.requests}</TableCell>
                      <TableCell className="right mono">{row.errors}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : <Alert><AlertDescription>{t("model.no_calls")}</AlertDescription></Alert>}
        </CardContent>
      </Card>
    </section>
  );
}
