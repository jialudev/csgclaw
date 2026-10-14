import { Combobox } from "@/components/ui/Combobox";
import { ModelOptionLabel } from "@/components/business/ProfileControls";
import type { TranslateFn } from "@/models/conversations";

export type ModelPickerProps = {
  value: string;
  models: readonly string[];
  disabled?: boolean;
  required?: boolean;
  placeholder: string;
  label?: string;
  unavailableLabel?: string;
  onChange: (value: string) => void;
  t: TranslateFn;
};

export function ModelPicker({
  value,
  models,
  disabled,
  required,
  placeholder,
  label,
  unavailableLabel,
  onChange,
  t,
}: ModelPickerProps) {
  const uniqueModels = [...new Set(models)];
  return (
    <Combobox
      value={value}
      disabled={disabled}
      required={required}
      onValueChange={onChange}
      triggerProps={{ "aria-label": label ?? t("profileModel"), "aria-required": required }}
      searchPlaceholder={t("modelProviderModelSearch")}
      emptyLabel={t("modelProviderNoModels")}
      options={[
        { value: "", label: placeholder },
        ...uniqueModels.map((model) => ({
          value: model,
          label: <ModelOptionLabel model={model} showAvatar={false} />,
          textValue: model,
        })),
        ...(value && !uniqueModels.includes(value)
          ? [
              {
                value,
                label: <ModelOptionLabel model={value} showAvatar={false} />,
                textValue: value,
                disabled: true,
                description: unavailableLabel ?? t("profileModelCurrentSelectionRetained"),
              },
            ]
          : []),
      ]}
    />
  );
}
