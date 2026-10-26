import { scheduleClock, scheduleInstant } from "./scheduleTime";

test("schedule fields use the saved timezone instead of the browser timezone", () => {
  expect(scheduleInstant("2026-10-27", "09:00", "Asia/Shanghai")).toBe("2026-10-27T01:00:00.000Z");
  expect(scheduleClock("2026-10-27T01:00:00Z", "Asia/Shanghai")).toEqual({ date: "2026-10-27", time: "09:00" });
  expect(scheduleInstant("2026-03-08", "09:00", "America/New_York")).toBe("2026-03-08T13:00:00.000Z");
});
test("missing, ambiguous and invalid wall-clock times are rejected", () => {
  expect(scheduleInstant("2026-03-08", "02:30", "America/New_York")).toBe("");
  expect(scheduleInstant("2026-11-01", "01:30", "America/New_York")).toBe("");
  expect(scheduleInstant("2026-02-31", "09:00", "Asia/Shanghai")).toBe("");
  expect(scheduleInstant("2026-10-27", "09:00", "wrong/zone")).toBe("");
});
