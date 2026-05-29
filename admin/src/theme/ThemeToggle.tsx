import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import type { ReactNode } from "react";
import { Button } from "../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import { useLocale } from "../i18n/locale";

const themeIcons: Record<"light" | "dark" | "system", ReactNode> = {
  light: <Sun className="size-4" />,
  dark: <Moon className="size-4" />,
  system: <Monitor className="size-4" />,
};

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const { t } = useLocale();
  const selectedTheme = theme === "light" || theme === "dark" || theme === "system" ? theme : "system";
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" type="button" aria-label={t("theme.title")} title={t("theme.title")}>
          {themeIcons[selectedTheme]}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        onCloseAutoFocus={(event) => event.preventDefault()}
      >
        <DropdownMenuRadioGroup className="grid gap-1" value={selectedTheme} onValueChange={(value) => setTheme(value)}>
          <DropdownMenuRadioItem value="light"><Sun /> {t("theme.light")}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark"><Moon /> {t("theme.dark")}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system"><Monitor /> {t("theme.system")}</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
