import { Pause, Play } from "lucide-react";
import { Button } from "./ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";
import { useLocale } from "../i18n/locale";

export function EnabledToggleButton(props: {
  enabled: boolean;
  disabled?: boolean;
  size?: "icon" | "icon-sm";
  onClick: () => void;
}) {
  const { t } = useLocale();
  const label = props.enabled ? t("common.disable") : t("common.enable");
  const Icon = props.enabled ? Pause : Play;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size={props.size ?? "icon"}
          disabled={props.disabled}
          aria-label={label}
          onClick={props.onClick}
        >
          <Icon />
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
