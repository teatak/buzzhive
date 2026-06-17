import { useEffect, useRef, useState, type PointerEvent } from "react";
import { addMinutes, isoMinute } from "../../lib/date";
import type { UsagePoint } from "../../types/admin";

const chartPlotLeftOffset = 52;
const chartPlotRightOffset = 8;
type CoordinateMode = "point" | "band";

export function useUsageRangeSelection(props: {
  series: UsagePoint[];
  bucketMinutes?: number;
  onRangeSelect?: (from: string, to: string) => void;
  coordinateMode?: CoordinateMode;
}) {
  const [dragStart, setDragStart] = useState<number | null>(null);
  const [dragEnd, setDragEnd] = useState<number | null>(null);
  const [isPointerSelecting, setIsPointerSelecting] = useState(false);
  const [suppressTooltip, setSuppressTooltip] = useState(false);
  const dragOrigin = useRef<{ index: number; x: number; pointerId: number } | null>(null);
  const preserveTooltipSuppression = useRef(false);
  const chartStateKey = `${props.series[0]?.date ?? "empty"}:${props.series[props.series.length - 1]?.date ?? "empty"}:${props.series.length}`;
  const bucketMinutes = props.bucketMinutes ?? 1;
  const isDaily = bucketMinutes >= 1440;
  const coordinateMode = props.coordinateMode ?? "point";

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
    const width = Math.max(1, rect.width - chartPlotLeftOffset - chartPlotRightOffset);
    const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left - chartPlotLeftOffset) / width));
    return Math.round(ratio * (props.series.length - 1));
  }

  function startSelection(event: PointerEvent<HTMLDivElement>) {
    if (!props.onRangeSelect) return;
    if (event.pointerType === "mouse" && event.button !== 0) return;
    if (props.series.length < 2) return;
    const index = indexForPointer(event);
    dragOrigin.current = { index, x: event.clientX, pointerId: event.pointerId };
    setIsPointerSelecting(true);
    setSuppressTooltip(true);
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function moveSelection(event: PointerEvent<HTMLDivElement>) {
    const origin = dragOrigin.current;
    if (!origin) {
      if (suppressTooltip) setSuppressTooltip(false);
      return;
    }
    event.stopPropagation();
    const index = indexForPointer(event);
    if (dragStart === null) {
      if (Math.abs(event.clientX - origin.x) < 4) return;
      setDragStart(origin.index);
    }
    setDragEnd(index);
  }

  function finishSelection(event: PointerEvent<HTMLDivElement>) {
    const wasSelecting = dragOrigin.current !== null;
    dragOrigin.current = null;
    setIsPointerSelecting(false);
    if (wasSelecting) event.stopPropagation();
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
    if (start === end && !isDaily) {
      setSuppressTooltip(false);
      setDragStart(null);
      setDragEnd(null);
      return;
    }
    preserveTooltipSuppression.current = true;
    props.onRangeSelect?.(isoMinute(new Date(props.series[start].date)), isoMinute(addMinutes(new Date(props.series[end].date), bucketMinutes)));
    setDragStart(null);
    setDragEnd(null);
  }

  const hasSelection = dragStart !== null && dragEnd !== null;
  const hideTooltip = isPointerSelecting || suppressTooltip;
  const selectionStart = hasSelection ? Math.min(dragStart, dragEnd) : 0;
  const selectionEnd = hasSelection ? Math.max(dragStart, dragEnd) : 0;
  const selectionLeft = hasSelection ? (isDaily ? pointBandLeft(selectionStart, props.series.length, coordinateMode) : (selectionStart / Math.max(1, props.series.length - 1)) * 100) : 0;
  const selectionRight = hasSelection ? (isDaily ? pointBandRight(selectionEnd, props.series.length, coordinateMode) : (selectionEnd / Math.max(1, props.series.length - 1)) * 100) : 0;
  const selectionWidth = Math.max(0, selectionRight - selectionLeft);

  return {
    chartStateKey,
    hasSelection,
    hideTooltip,
    selectionStyle: { left: `${selectionLeft}%`, width: `${selectionWidth}%`, position: "absolute" as const },
    handlers: {
      onPointerDownCapture: startSelection,
      onPointerMoveCapture: moveSelection,
      onPointerUpCapture: finishSelection,
      onPointerCancelCapture: finishSelection,
      onLostPointerCapture: () => {
        dragOrigin.current = null;
        setIsPointerSelecting(false);
        setSuppressTooltip(false);
        setDragStart(null);
        setDragEnd(null);
      },
      onPointerLeave: () => setSuppressTooltip(false),
    },
  };
}

function pointBandLeft(index: number, pointCount: number, coordinateMode: CoordinateMode) {
  if (pointCount <= 1) return 0;
  if (coordinateMode === "band") {
    return (index / pointCount) * 100;
  }
  if (index === 0) return 0;
  return (pointCenter(index - 1, pointCount) + pointCenter(index, pointCount)) / 2;
}

function pointBandRight(index: number, pointCount: number, coordinateMode: CoordinateMode) {
  if (pointCount <= 1) return 100;
  if (coordinateMode === "band") {
    return ((index + 1) / pointCount) * 100;
  }
  if (index === pointCount - 1) return 100;
  return (pointCenter(index, pointCount) + pointCenter(index + 1, pointCount)) / 2;
}

function pointCenter(index: number, pointCount: number) {
  return (index / Math.max(1, pointCount - 1)) * 100;
}
