type SparklineProps = {
  data: number[];
  stroke?: string;
  height?: number;
  width?: number;
};

/**
 * Zero-dep SVG sparkline. Renders a simple polyline path.
 * Marked aria-hidden — purely decorative trend indicator.
 * Returns null if fewer than 2 data points (no line to draw).
 */
export function Sparkline({
  data,
  stroke = "currentColor",
  height = 32,
  width = 96,
}: SparklineProps) {
  if (data.length < 2) return null;

  const max = Math.max(...data, 1);
  const min = Math.min(...data, 0);
  const range = max - min || 1;
  const stepX = width / Math.max(data.length - 1, 1);

  const points = data
    .map((v, i) => {
      const x = i * stepX;
      const y = height - ((v - min) / range) * height;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
      <title>xu hướng 7 ngày</title>
      <polyline
        points={points}
        fill="none"
        stroke={stroke}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
