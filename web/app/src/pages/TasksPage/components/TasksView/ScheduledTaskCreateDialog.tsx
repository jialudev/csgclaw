import { useState } from "react";
import { ScheduleNaturalInput } from "./ScheduleNaturalInput";
import {
  Button,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
} from "@/components/ui";
import { classNames } from "@/shared/lib/classNames";
import { ScheduledTaskFormFields } from "./ScheduledTaskFormFields";
import { TaskDialogCloseButton } from "./TaskDialogCloseButton";
import styles from "./TasksView.module.css";
import type { Dispatch, SetStateAction } from "react";
import type { SelectOption } from "@/components/ui";
import type { TranslateFn } from "@/models/conversations";
import type { ScheduledTaskFormDraft, ScheduledTaskFormFieldErrors } from "./taskDialogTypes";

type ScheduledTaskCreateDialogProps = {
  busy: boolean;
  draft: ScheduledTaskFormDraft;
  error: string;
  errors: ScheduledTaskFormFieldErrors;
  onChange: Dispatch<SetStateAction<ScheduledTaskFormDraft>>;
  onClearError: (field: keyof ScheduledTaskFormFieldErrors) => void;
  onClose?: () => void;
  onSubmit: () => void | Promise<void>;
  open: boolean;
  scheduledAgentOptions: readonly SelectOption[];
  t: TranslateFn;
};

export function ScheduledTaskCreateDialog({
  busy,
  draft,
  error,
  errors,
  open,
  scheduledAgentOptions,
  t,
  onChange,
  onClearError,
  onClose,
  onSubmit,
}: ScheduledTaskCreateDialogProps) {
  const [parsing, setParsing] = useState(false);
  return (
    <DialogRoot open={open} onOpenChange={(nextOpen) => (!nextOpen && !parsing ? onClose?.() : null)}>
      <DialogContent className={classNames(styles.taskCreateDialog, styles.scheduledCreateDialog)}>
        <DialogHeader>
          <div>
            <DialogTitle>{t("scheduledTaskCreateTitle")}</DialogTitle>
            <DialogDescription>{t("scheduledTaskCreateSubtitle")}</DialogDescription>
          </div>
          <TaskDialogCloseButton label={t("close")} />
        </DialogHeader>
        <DialogBody className={styles.scheduledCreateBody}>
          <ScheduleNaturalInput
            draft={draft}
            onChange={(value) => {
              onChange(value);
              for (const field of Object.keys(errors) as (keyof ScheduledTaskFormFieldErrors)[]) onClearError(field);
            }}
            onBusyChange={setParsing}
            disabled={busy || parsing}
            t={t}
          />
          <fieldset className={styles.schedulePreviewFields} disabled={parsing || busy}>
            <ScheduledTaskFormFields
              draft={draft}
              errors={errors}
              scheduledAgentOptions={scheduledAgentOptions}
              t={t}
              onChange={onChange}
              onClearError={onClearError}
            />
          </fieldset>
          {error ? <div className={classNames("form-error", styles.taskCreateError)}>{error}</div> : null}
        </DialogBody>
        <DialogFooter className={styles.scheduledCreateFooter}>
          <Button variant="secondaryGray" size="md" disabled={parsing || busy} onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={busy}
            loadingLabel={t("scheduledTaskCreating")}
            disabled={busy || parsing}
            onClick={onSubmit}
          >
            {t("scheduledTaskCreateSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </DialogRoot>
  );
}
