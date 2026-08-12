export const LEVELS = [
  ["error", "Error", "#ff4242"],
  ["warn", "Warn", "#ff8a00"],
  ["info", "Info", "#87ceeb"],
  ["debug", "Debug", "#178f15"],
  ["meta", "Meta", "#4e49da"],
  ["other", "Other", "#94a3b8"],
];

export const PERIODS = [
  ["hour", "Hour"],
  ["day", "Day"],
  ["month", "Month"],
];

// "now" is the unflushed interval, so it is pinned right rather than sorted.
export function orderBuckets(buckets = {}) {
  const keys = Object.keys(buckets);
  return keys
    .filter((b) => b !== "now")
    .sort()
    .concat(keys.includes("now") ? ["now"] : []);
}

export function bucketLabel(bucket, unit) {
  if (bucket === "now") {
    return "now";
  }
  const date = new Date(bucket);
  if (Number.isNaN(date.getTime())) {
    return bucket;
  }
  if (unit === "hour") {
    return date.toLocaleString([], { hour: "2-digit", minute: "2-digit" });
  }
  if (unit === "day") {
    return date.toLocaleString([], { month: "short", day: "2-digit" });
  }
  return date.toLocaleString([], { month: "short", year: "numeric" });
}

export function totalLines(buckets = {}) {
  return orderBuckets(buckets).reduce(
    (sum, b) => sum + LEVELS.reduce((n, [key]) => n + (buckets[b]?.[key] ?? 0), 0),
    0
  );
}

export function toChartData(buckets = {}, unit = "hour") {
  const ordered = orderBuckets(buckets);
  return {
    labels: ordered.map((b) => bucketLabel(b, unit)),
    datasets: LEVELS.map(([key, name, colour]) => ({
      label: name,
      data: ordered.map((b) => buckets[b]?.[key] ?? 0),
      stack: "Stack 0",
      backgroundColor: colour,
      borderWidth: 1,
    })),
  };
}
