import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "../../components/ui/chart";
import { tNow } from "../../i18n/locale";
import type { UsagePoint } from "../../types/admin";
import { formatCompactNumber } from "../../lib/utils";
import { ChartDayBands } from "./ChartDayBands";
import { useUsageRangeSelection } from "./useUsageRangeSelection";

export function UsageChart(props: { series: UsagePoint[]; bucketMinutes?: number; onRangeSelect?: (from: string, to: string) => void }) {
  const selection = useUsageRangeSelection(props);

  return (
    <div
      className="relative cursor-col-resize select-none"
      {...selection.handlers}
    >
      {selection.hasSelection && (
        <div className="pointer-events-none absolute top-3 right-2 bottom-7 left-[3.25rem] z-20">
          <div
            className="h-full rounded-sm bg-primary/15 ring-1 ring-primary/35"
            style={selection.selectionStyle}
          />
        </div>
      )}
      <ChartDayBands series={props.series} bucketMinutes={props.bucketMinutes} />
      <ChartContainer
        className="relative z-10 h-60 w-full"
        config={{
          requests: { label: tNow("dashboard.requests"), color: "var(--primary)" },
        }}
      >
        <AreaChart key={selection.chartStateKey} data={props.series} margin={{ left: 8, right: 8, top: 12, bottom: 0 }}>
          <CartesianGrid vertical={false} />
          <XAxis dataKey="label" tickLine={false} axisLine={false} minTickGap={32} />
          <YAxis tickLine={false} axisLine={false} width={44} allowDecimals={false} tickFormatter={formatCompactNumber} />
          <ChartTooltip
            isAnimationActive={false}
            animationDuration={0}
            active={selection.hideTooltip ? false : undefined}
            cursor={selection.hideTooltip ? false : undefined}
            wrapperStyle={{ transition: "none", visibility: selection.hideTooltip ? "hidden" : undefined }}
            content={
              <ChartTooltipContent
                labelFormatter={(_, payload) => {
                  const point = payload?.[0]?.payload as UsagePoint | undefined;
                  return point?.tooltip ?? point?.label ?? "";
                }}
                formatter={(value) => (
                  <div className="flex flex-1 justify-between leading-none items-center gap-2">
                    <span className="text-muted-foreground">{tNow("dashboard.requests")}</span>
                    <span className="font-mono font-medium text-foreground tabular-nums">
                      {formatCompactNumber(Number(value))}
                    </span>
                  </div>
                )}
              />
            }
          />
          <Area dataKey="requests" type="monotone" fill="var(--color-requests)" fillOpacity={0.12} stroke="var(--color-requests)" strokeWidth={2} activeDot={selection.hideTooltip ? false : undefined} animationDuration={180} />
        </AreaChart>
      </ChartContainer>
    </div>
  );
}
