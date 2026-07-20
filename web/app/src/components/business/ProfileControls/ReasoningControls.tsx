import { useId } from "react";
import { Select } from "@/components/ui";
import { REASONING_OPTIONS } from "@/shared/constants/agents";
import { normalizeReasoningEffort } from "@/models/reasoning";
import type { TranslateFn } from "@/models/conversations";
import { reasoningOptionMessageKeys } from "./reasoningLabels";

export type ReasoningControlsProps = {
  onChange: (value: string) => void;
  t: TranslateFn;
  value: string;
};

export function ReasoningControls({ onChange, t, value }: ReasoningControlsProps) {
  const helpID = useId();

  return (
    <div className="field reasoning-controls-field">
      <span>{t("profileReasoning")}</span>
      <Select
        value={normalizeReasoningEffort(value)}
        onValueChange={onChange}
        triggerProps={{ "aria-label": t("profileReasoningEffort"), "aria-describedby": helpID }}
        options={REASONING_OPTIONS.map((option) => ({
          value: option,
          label: t(reasoningOptionMessageKeys[option]),
        }))}
      />
      <small id={helpID} className="reasoning-controls-help">
        {t("profileReasoningHelp")}
      </small>
    </div>
  );
}
