import type { UsagePoint } from "../../types/admin";

type DayBand = {
  key: string;
  left: number;
  width: number;
};

type CoordinateMode = "point" | "band";

export function ChartDayBands(props: { series: UsagePoint[]; bucketMinutes?: number; coordinateMode?: CoordinateMode }) {
  const isDaily = (props.bucketMinutes ?? 1) >= 1440;
  const coordinateMode = props.coordinateMode ?? "point";
  const bands = isDaily ? buildCenteredDayBands(props.series, coordinateMode) : buildBoundaryDayBands(props.series, coordinateMode);

  return (
    <div className="pointer-events-none absolute top-3 right-2 bottom-7 left-[3.25rem] z-0 overflow-hidden rounded-sm">
      {bands.map((band) => (
        <div
          key={band.key}
          className="absolute inset-y-0 bg-muted/80 dark:bg-muted/35"
          style={{ left: `${band.left}%`, width: `${band.width}%` }}
        />
      ))}
    </div>
  );
}

function buildCenteredDayBands(series: UsagePoint[], coordinateMode: CoordinateMode): DayBand[] {
  const points = series
    .map((point, index) => {
      const date = new Date(point.date);
      return { dayKey: localDayKey(date), index, valid: Number.isFinite(date.getTime()) };
    })
    .filter((point) => point.valid);

  if (points.length < 2) return [];

  const bands: DayBand[] = [];
  let currentDay = points[0].dayKey;
  let currentDayIndex = 0;
  let currentLeft = pointBandLeft(points[0].index, series.length, coordinateMode);
  let currentRight = pointBandRight(points[0].index, series.length, coordinateMode);

  for (const point of points.slice(1)) {
    const left = pointBandLeft(point.index, series.length, coordinateMode);
    const right = pointBandRight(point.index, series.length, coordinateMode);
    if (point.dayKey === currentDay) {
      currentRight = right;
      continue;
    }

    if (currentDayIndex % 2 === 1) {
      bands.push({
        key: currentDay,
        left: currentLeft,
        width: currentRight - currentLeft,
      });
    }

    currentDay = point.dayKey;
    currentDayIndex += 1;
    currentLeft = left;
    currentRight = right;
  }

  if (currentDayIndex % 2 === 1) {
    bands.push({
      key: currentDay,
      left: currentLeft,
      width: currentRight - currentLeft,
    });
  }

  return bands;
}

function buildBoundaryDayBands(series: UsagePoint[], coordinateMode: CoordinateMode): DayBand[] {
  const dayStarts = series
    .map((point, index) => {
      const date = new Date(point.date);
      return { dayKey: localDayKey(date), index, valid: Number.isFinite(date.getTime()) };
    })
    .filter((point, index, points) => point.valid && point.dayKey !== points[index - 1]?.dayKey);

  if (series.length < 2 || dayStarts.length < 2) return [];

  const bands: DayBand[] = [];
  for (let index = 0; index < dayStarts.length; index += 1) {
    if (index % 2 === 1) {
      const dayStart = dayStarts[index];
      const nextDayStart = dayStarts[index + 1];
      const left = dayBoundary(dayStart.index, series.length, coordinateMode);
      const right = nextDayStart ? dayBoundary(nextDayStart.index, series.length, coordinateMode) : 100;
      bands.push({
        key: dayStart.dayKey,
        left,
        width: right - left,
      });
    }
  }
  return bands;
}

function localDayKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}`;
}

function dayBoundary(index: number, pointCount: number, coordinateMode: CoordinateMode) {
  if (coordinateMode === "band") return pointBandLeft(index, pointCount, coordinateMode);
  return pointCenter(index, pointCount);
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
