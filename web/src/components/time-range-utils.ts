export type TimeRangePreset = "custom" | "today" | "yesterday" | "last24h" | "last7" | "last14" | "last30" | "month" | "last_month";

export type TimeRangeValue = {
  preset: TimeRangePreset;
  from: string;
  to: string;
};

export function timeRangeQuery(value: TimeRangeValue) {
  return {
    timeRange: value.preset === "custom" ? undefined : value.preset,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    from: value.preset === "custom" && value.from ? new Date(`${value.from}T00:00:00`).toISOString() : undefined,
    to: value.preset === "custom" && value.to ? new Date(`${value.to}T23:59:59.999`).toISOString() : undefined,
  };
}
