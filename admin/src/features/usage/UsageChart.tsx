import { useEffect, useRef, useState, type PointerEvent } from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "../../components/ui/chart";
import { tNow } from "../../i18n/locale";
import { addMinutes, isoMinute } from "../../lib/date";
import type { UsagePoint } from "../../types/admin";

export function UsageChart(props: { series: UsagePoint[]; bucketMinutes?: number; onRangeSelect?: (from: string, to: string) => void }) {
  const [dragStart, setDragStart] = useState<number | null>(null);
  const [dragEnd, setDragEnd] = useState<number | null>(null);
  const [isPointerSelecting, setIsPointerSelecting] = useState(false);
  const [suppressTooltip, setSuppressTooltip] = useState(false);
  const dragOrigin = useRef<{ index: number; x: number; pointerId: number } | null>(null);
  const preserveTooltipSuppression = useRef(false);
  const chartStateKey = `${props.series[0]?.date ?? "empty"}:${props.series[props.series.length - 1]?.date ?? "empty"}:${props.series.length}`;

  useEffect(() => {
    dragOrigin.current = null;
    setDragStart(null);
    setDragEnd(null);
    setIsPointerSelecting(false);
    if (preserveTooltipSuppression.current) {
      preserveTooltipSuppression.current = false;
    } else {
      setSuppressTooltip(false);
    }
  }, [chartStateKey]);

  function indexForPointer(event: PointerEvent<HTMLDivElement>) {
    const rect = event.currentTarget.getBoundingClientRect();
    const leftOffset = 36;
    const rightOffset = 8;
    const width = Math.max(1, rect.width - leftOffset - rightOffset);
    const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left - leftOffset) / width));
    return Math.round(ratio * (props.series.length - 1));
  }

  function startSelection(event: PointerEvent<HTMLDivElement>) {
    if (props.series.length < 2) return;
    const index = indexForPointer(event);
    dragOrigin.current = { index, x: event.clientX, pointerId: event.pointerId };
    setIsPointerSelecting(true);
    setSuppressTooltip(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function moveSelection(event: PointerEvent<HTMLDivElement>) {
    const origin = dragOrigin.current;
    if (!origin) {
      if (suppressTooltip) setSuppressTooltip(false);
      return;
    }
    const index = indexForPointer(event);
    if (dragStart === null) {
      if (Math.abs(event.clientX - origin.x) < 4) return;
      setDragStart(origin.index);
    }
    setDragEnd(index);
  }

  function finishSelection(event: PointerEvent<HTMLDivElement>) {
    dragOrigin.current = null;
    setIsPointerSelecting(false);
    if (dragStart === null || dragEnd === null) {
      setSuppressTooltip(false);
      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId);
      }
      return;
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    const start = Math.min(dragStart, dragEnd);
    const end = Math.max(dragStart, dragEnd);
    if (start !== end) {
      preserveTooltipSuppression.current = true;
      props.onRangeSelect?.(props.series[start].date, isoMinute(addMinutes(new Date(props.series[end].date), props.bucketMinutes ?? 1)));
    } else {
      setSuppressTooltip(false);
    }
    setDragStart(null);
    setDragEnd(null);
  }

  const hasSelection = dragStart !== null && dragEnd !== null;
  const hideTooltip = isPointerSelecting || suppressTooltip;
  const selectionStart = hasSelection ? Math.min(dragStart, dragEnd) : 0;
  const selectionEnd = hasSelection ? Math.max(dragStart, dragEnd) : 0;
  const selectionLeft = hasSelection ? (selectionStart / Math.max(1, props.series.length - 1)) * 100 : 0;
  const selectionWidth = hasSelection ? ((selectionEnd - selectionStart) / Math.max(1, props.series.length - 1)) * 100 : 0;

  return (
    <div
      className="relative cursor-col-resize select-none"
      onPointerDown={startSelection}
      onPointerMove={moveSelection}
      onPointerUp={finishSelection}
      onPointerCancel={finishSelection}
      onLostPointerCapture={() => {
        dragOrigin.current = null;
        setIsPointerSelecting(false);
        setSuppressTooltip(false);
        setDragStart(null);
        setDragEnd(null);
      }}
      onPointerLeave={() => setSuppressTooltip(false)}
    >
      {hasSelection && (
        <div className="pointer-events-none absolute top-3 right-2 bottom-7 left-9 z-10">
          <div
            className="h-full rounded-sm bg-primary/15 ring-1 ring-primary/35"
            style={{ left: `${selectionLeft}%`, width: `${selectionWidth}%`, position: "absolute" }}
          />
        </div>
      )}
      <ChartContainer
        className="h-60 w-full"
        config={{ requests: { label: tNow("dashboard.requests"), color: "var(--primary)" } }}
      >
        <AreaChart key={chartStateKey} data={props.series} margin={{ left: 8, right: 8, top: 12, bottom: 0 }}>
          <CartesianGrid vertical={false} />
          <XAxis dataKey="date" tickLine={false} axisLine={false} minTickGap={32} />
          <YAxis tickLine={false} axisLine={false} width={28} allowDecimals={false} />
          <ChartTooltip
            active={hideTooltip ? false : undefined}
            cursor={hideTooltip ? false : undefined}
            wrapperStyle={{ transition: "none", visibility: hideTooltip ? "hidden" : undefined }}
            content={<ChartTooltipContent />}
          />
          <Area
            dataKey="requests"
            type="monotone"
            activeDot={hideTooltip ? false : undefined}
            animationDuration={180}
            animationEasing="ease-out"
            fill="var(--color-requests)"
            fillOpacity={0.18}
            stroke="var(--color-requests)"
            strokeWidth={2}
          />
        </AreaChart>
      </ChartContainer>
    </div>
  );
}
