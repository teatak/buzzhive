import type { ReactNode } from "react";
import type { UsagePoint } from "../types/admin";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";

export function UsageChart(props: { series: UsagePoint[] }) {
  const width = 720;
  const height = 220;
  const pad = 28;
  const max = Math.max(1, ...props.series.map((point) => point.requests));
  const points = props.series.map((point, index) => {
    const x = props.series.length <= 1 ? pad : pad + (index * (width - pad * 2)) / (props.series.length - 1);
    const y = height - pad - (point.requests / max) * (height - pad * 2);
    return { ...point, x, y };
  });
  const barWidth = Math.max(1, Math.min(18, ((width - pad * 2) / Math.max(points.length, 1)) * 0.6));
  const line = points.map((point) => `${point.x},${point.y}`).join(" ");
  return (
    <div className="chart-wrap">
      <svg className="usage-chart" viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Usage chart">
        <line x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} />
        <line x1={pad} y1={pad} x2={pad} y2={height - pad} />
        {points.map((point) => (
          <g key={point.date}>
            <rect
              x={point.x - barWidth / 2}
              y={point.y}
              width={barWidth}
              height={height - pad - point.y}
              rx="3"
            />
          </g>
        ))}
        <polyline points={line} />
        {points.length <= 240 && points.map((point) => <circle key={`${point.date}-dot`} cx={point.x} cy={point.y} r="3.5" />)}
      </svg>
      <div className="chart-axis">
        <span>{props.series[0]?.date ?? "-"}</span>
        <span>{props.series[props.series.length - 1]?.date ?? "-"}</span>
      </div>
    </div>
  );
}

export function UsageByKey(props: { usage: Record<string, number> }) {
  const rows = Object.entries(props.usage).sort((a, b) => b[1] - a[1]);
  if (!rows.length) return <div className="empty">No usage</div>;
  return (
    <div className="list">{rows.map(([key, count]) => (
      <div className="list-row" key={key}>
        <span className="mono">{key}</span>
        <span className="mono">{count}</span>
      </div>
    ))}</div>
  );
}

export function Metric(props: { icon: ReactNode; label: string; value: ReactNode }) {
  return <section className="metric-card"><div className="metric-label">{props.icon}{props.label}</div><div className="metric-value">{props.value}</div></section>;
}

export function Panel(props: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
        {props.action}
      </CardHeader>
      <CardContent>{props.children}</CardContent>
    </Card>
  );
}

export function InfoRow(props: { label: string; value: string }) {
  return <div className="list-row"><span className="muted">{props.label}</span><span className="mono">{props.value}</span></div>;
}
