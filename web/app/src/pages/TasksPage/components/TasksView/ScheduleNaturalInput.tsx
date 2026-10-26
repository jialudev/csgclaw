import styles from "./TasksView.module.css";
import { useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import { Button, Field, TextArea } from "@/components/ui";
import { parseScheduledTask } from "@/api/scheduledTasks";
import { errorMessage } from "@/api/client";
import { browserTimeZone, scheduleClock } from "@/models/scheduleTime";
import type { TranslateFn } from "@/models/conversations";
import type { ScheduledTaskFormDraft } from "./taskDialogTypes";

export function ScheduleNaturalInput({
  draft,
  onChange,
  onBusyChange,
  disabled,
  t,
}: {
  draft: ScheduledTaskFormDraft;
  onChange: Dispatch<SetStateAction<ScheduledTaskFormDraft>>;
  onBusyChange: (value: boolean) => void;
  disabled: boolean;
  t: TranslateFn;
}) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const inFlight = useRef(false);
  async function parse() {
    if (inFlight.current || !text.trim()) return;
    inFlight.current = true;
    setBusy(true);
    onBusyChange(true);
    setMessage("");
    try {
      const preview = await parseScheduledTask(text, draft.timeZone || browserTimeZone(), draft.agentID || undefined);
      if (preview.clarification) {
        setMessage(preview.clarification);
        return;
      }
      const clock = scheduleClock(preview.first_run_at, preview.time_zone);
      onChange({
        title: preview.title,
        prompt: preview.prompt,
        agentID: preview.agent_id,
        recurrence: preview.recurrence,
        date: clock.date,
        time: clock.time,
        timeZone: preview.time_zone,
        clientRequestID: preview.client_request_id,
        expiresDate: scheduleClock(preview.expires_at || "", preview.time_zone).date,
      });
      setMessage(t("schedulePreviewReady"));
    } catch (error) {
      setMessage(errorMessage(error, t("scheduleParseFailed")));
    } finally {
      inFlight.current = false;
      setBusy(false);
      onBusyChange(false);
    }
  }
  return (
    <div className={styles.scheduleNatural}>
      <Field label={t("scheduleNaturalLabel")} hint={t("scheduleNaturalHint")}>
        <TextArea
          className={styles.scheduleNaturalText}
          rows={2}
          value={text}
          disabled={disabled || busy}
          placeholder={t("scheduleNaturalPlaceholder")}
          onChange={(event) => setText(event.currentTarget.value)}
        />
      </Field>
      <Button
        variant="secondaryGray"
        size="sm"
        loading={busy}
        disabled={disabled || busy || !text.trim()}
        onClick={() => void parse()}
      >
        {t("scheduleParse")}
      </Button>
      {message ? <p role="status">{message}</p> : null}
    </div>
  );
}
