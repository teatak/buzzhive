import { useId } from "react";
import { Settings2 } from "lucide-react";
import { BrandIcon, brandIconName } from "./brand-icons";
import { LabelWithTip } from "./form-fields";
import { Field } from "./ui/field";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";
import { useLocale } from "../i18n/locale";
import { modelIconName, modelIconOptions } from "../lib/model-icon";
import { cn } from "../lib/utils";
import type { Model, ModelPreset } from "../types/admin";

type IconModel = Pick<Model, "name" | "display_name" | "icon">;
type IconPreset = Pick<ModelPreset, "name" | "family">;

function IconGlyph({ name, className }: { name: string; className?: string }) {
  const brand = brandIconName(name);
  return (
    <span aria-hidden="true" data-model-icon={brand || "generic"} className={cn("inline-flex shrink-0", className)}>
      {brand ? <BrandIcon name={brand} className="size-full rounded-[inherit]" /> : (
        <span className="flex size-full items-center justify-center rounded-[inherit] border border-dashed text-muted-foreground">
          <Settings2 className="!size-1/2" strokeWidth={2} />
        </span>
      )}
    </span>
  );
}

export function ModelIcon({ model, presets, className = "h-10 w-10" }: { model: IconModel; presets: IconPreset[]; className?: string }) {
  return <IconGlyph name={modelIconName(model, presets)} className={cn("rounded-[10px]", className)} />;
}

export function ModelPresetIcon({ preset, className = "h-8 w-8 rounded-[8px]" }: { preset: IconPreset; className?: string }) {
  return <IconGlyph name={preset.family} className={className} />;
}

export function ModelIconField({ model, presets, onChange }: { model: IconModel; presets: IconPreset[]; onChange: (icon: string) => void }) {
  const id = useId();
  const { t } = useLocale();
  const automaticBrand = brandIconName(modelIconName({ ...model, icon: "" }, presets));
  const automaticLabel = modelIconOptions.find((item) => item.value === automaticBrand)?.label ?? t("models.icon_generic");
  return (
    <Field>
      <LabelWithTip htmlFor={id} label={t("models.icon")} tip={t("models.tip_icon")} />
      <Select value={model.icon || "auto"} onValueChange={(value) => onChange(value === "auto" ? "" : value)}>
        <SelectTrigger id={id} className="h-10 w-full"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="auto">
            <IconGlyph name={automaticBrand} className="size-6 rounded-md" />
            <span>{t("models.icon_auto")} · {automaticLabel}</span>
          </SelectItem>
          {modelIconOptions.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              <IconGlyph name={option.value} className="size-6 rounded-md" />
              <span>{option.label}</span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}
