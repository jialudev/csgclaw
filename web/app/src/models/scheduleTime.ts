export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export function scheduleClock(value: string, timeZone = browserTimeZone()): { date: string; time: string } {
  if (!value) return { date: "", time: "" };
  try {
    const parts = new Intl.DateTimeFormat("en-GB", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    }).formatToParts(new Date(value));
    const part = (name: string) => parts.find((p) => p.type === name)?.value || "";
    return { date: `${part("year")}-${part("month")}-${part("day")}`, time: `${part("hour")}:${part("minute")}` };
  } catch {
    return { date: "", time: "" };
  }
}

// Convert the displayed wall-clock time without depending on the server/browser zone.
// Reject missing or repeated DST hours rather than silently moving the schedule.
export function scheduleInstant(date: string, time: string, timeZone = browserTimeZone()): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !/^\d{2}:\d{2}$/.test(time)) return "";
  const wall = Date.parse(`${date}T${time}:00Z`);
  if (!Number.isFinite(wall)) return "";
  let candidate = wall;
  for (let i = 0; i < 4; i++) {
    const clock = scheduleClock(new Date(candidate).toISOString(), timeZone);
    if (!clock.date) return "";
    const delta = wall - Date.parse(`${clock.date}T${clock.time}:00Z`);
    if (!delta) break;
    candidate += delta;
  }
  const matches = (value: number) => {
    const clock = scheduleClock(new Date(value).toISOString(), timeZone);
    return clock.date === date && clock.time === time;
  };
  if (!matches(candidate) || [-3600000, 3600000, -1800000, 1800000].some((offset) => matches(candidate + offset)))
    return "";
  return new Date(candidate).toISOString();
}
