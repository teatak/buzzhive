import { Check, Languages } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { LOCALE_LABEL, useLocale, type Locale } from "./locale";

const LOCALES: Locale[] = ["zh-CN", "zh-TW", "en"];

export function LocaleToggle() {
  const { locale, setLocale, t } = useLocale();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" type="button" aria-label={t("language.switch")} title={LOCALE_LABEL[locale]}>
          <Languages size={16} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="grid w-44 gap-1"
        onCloseAutoFocus={(event) => event.preventDefault()}
      >
        {LOCALES.map((item) => (
          <DropdownMenuItem key={item} onSelect={() => setLocale(item)}>
            <span>{LOCALE_LABEL[item]}</span>
            {item === locale && <Check className="ml-auto size-4" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
