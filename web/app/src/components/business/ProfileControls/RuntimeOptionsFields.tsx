import {
  localizedRuntimeOptionDescription,
  localizedRuntimeOptionLabel,
  runtimeOptionValueForPath,
  setRuntimeOptionValue,
  type AgentDraft,
  type RuntimeOptionSchema,
} from "@/models/agents";
import type { LocaleCode } from "@/models/conversations";

export type RuntimeOptionsFieldsProps = {
  draft: AgentDraft;
  locale: LocaleCode;
  schemas?: RuntimeOptionSchema[] | null;
  onDraftChange: (draft: AgentDraft) => void;
};

export function RuntimeOptionsFields({ draft, locale, schemas = [], onDraftChange }: RuntimeOptionsFieldsProps) {
  if (!Array.isArray(schemas) || schemas.length === 0) {
    return null;
  }

  return (
    <div className="profile-grid-compact">
      {schemas.map((schema) => {
        const path = String(schema.path ?? "").trim();
        if (!path) {
          return null;
        }
        const label = localizedRuntimeOptionLabel(schema, locale);
        const description = localizedRuntimeOptionDescription(schema, locale);
        const inputValue = runtimeOptionValueForPath(draft.runtime_options, path);
        const placeholder = schema.type === "directory" ? "/path/to/workspace" : "";
        return (
          <label key={String(schema.key ?? path)} className="field span-2">
            <span>{label}</span>
            <input
              value={inputValue}
              required={Boolean(schema.required)}
              aria-required={schema.required ? "true" : undefined}
              placeholder={placeholder}
              onInput={(event) =>
                onDraftChange({
                  ...draft,
                  runtime_options: setRuntimeOptionValue(draft.runtime_options, path, event.currentTarget.value),
                })
              }
            />
            {description ? <span className="field-hint">{description}</span> : null}
          </label>
        );
      })}
    </div>
  );
}
