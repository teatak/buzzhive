import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChartContainer, ChartTooltip } from "../../components/ui/chart";
import { tNow } from "../../i18n/locale";
import type { UsagePoint } from "../../types/admin";

type TokenUsagePoint = UsagePoint & {
  input_uncached_tokens: number;
  input_cached_tokens: number;
  output_text_tokens: number;
};

export function TokenUsageChart(props: { series: UsagePoint[] }) {
  const chartData = props.series.map(toTokenUsagePoint);

  return (
    <ChartContainer
      className="h-60 w-full"
      config={{
        input_cached_tokens: { label: tNow("usage.input_cached_tokens"), color: "var(--chart-1)" },
        input_uncached_tokens: { label: tNow("usage.input_uncached_tokens"), color: "var(--chart-2)" },
        reasoning_tokens: { label: tNow("usage.reasoning_tokens"), color: "oklch(0.56 0.2 32)" },
        output_text_tokens: { label: tNow("usage.output_text_tokens"), color: "oklch(0.82 0.18 82)" },
      }}
    >
      <BarChart data={chartData} margin={{ left: 8, right: 8, top: 12, bottom: 0 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="label" tickLine={false} axisLine={false} minTickGap={32} />
        <YAxis tickLine={false} axisLine={false} width={44} allowDecimals={false} />
        <ChartTooltip
          isAnimationActive={false}
          animationDuration={0}
          wrapperStyle={{ transition: "none" }}
          content={<TokenUsageTooltip />}
        />
        <Bar dataKey="input_cached_tokens" stackId="tokens" fill="var(--color-input_cached_tokens)" radius={[0, 0, 4, 4]} />
        <Bar dataKey="input_uncached_tokens" stackId="tokens" fill="var(--color-input_uncached_tokens)" />
        <Bar dataKey="reasoning_tokens" stackId="tokens" fill="var(--color-reasoning_tokens)" />
        <Bar dataKey="output_text_tokens" stackId="tokens" fill="var(--color-output_text_tokens)" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ChartContainer>
  );
}

function TokenUsageTooltip(props: {
  active?: boolean;
  payload?: Array<{
    dataKey?: string | number;
    value?: number | string;
    color?: string;
    fill?: string;
    payload?: TokenUsagePoint;
  }>;
}) {
  if (!props.active || !props.payload?.length) return null;
  const point = props.payload[0]?.payload;
  if (!point) return null;

  const labels: Record<string, string> = {
    input_uncached_tokens: tNow("usage.input_uncached_tokens"),
    input_cached_tokens: tNow("usage.input_cached_tokens"),
    output_text_tokens: tNow("usage.output_text_tokens"),
    reasoning_tokens: tNow("usage.reasoning_tokens"),
  };

  return (
    <div className="grid min-w-56 items-start gap-1.5 rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl">
      <div className="font-medium">{point.tooltip ?? point.label ?? ""}</div>
      <div className="my-0.5 h-px bg-border/60" />
      {props.payload.map((item) => {
        const key = String(item.dataKey ?? "");
        return (
          <TooltipRow
            key={key}
            color={item.fill ?? item.color}
            label={labels[key] ?? key}
            value={typeof item.value === "number" ? item.value : Number(item.value) || 0}
          />
        );
      })}
      <TooltipRow label={tNow("usage.total_tokens")} value={point.total_tokens} />
    </div>
  );
}

function TooltipRow(props: { label: string; value: number; color?: string }) {
  return (
    <div className="grid grid-cols-[10px_minmax(120px,1fr)_auto] items-center gap-2">
      {props.color ? <span className="h-2.5 w-2.5 shrink-0 rounded-[2px]" style={{ background: props.color }} /> : null}
      {!props.color ? <span /> : null}
      <span className="text-muted-foreground">{props.label}</span>
      <span className="text-right font-mono font-medium text-foreground tabular-nums">{props.value.toLocaleString()}</span>
    </div>
  );
}

function toTokenUsagePoint(point: UsagePoint): TokenUsagePoint {
  const cachedTokens = Math.max(0, point.cached_tokens);
  const promptTokens = Math.max(0, point.prompt_tokens);
  const reasoningTokens = Math.max(0, point.reasoning_tokens);
  const completionTokens = Math.max(0, point.completion_tokens);

  return {
    ...point,
    input_cached_tokens: Math.min(cachedTokens, promptTokens),
    input_uncached_tokens: Math.max(0, promptTokens - cachedTokens),
    output_text_tokens: Math.max(0, completionTokens - reasoningTokens),
    reasoning_tokens: reasoningTokens,
  };
}
