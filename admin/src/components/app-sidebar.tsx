import { Activity, Database, Gauge, KeyRound, UserRound } from "lucide-react";
import type { View } from "@/types/admin";
import { BrandLogo } from "@/components/brand-logo";
import { useLocale } from "@/i18n/locale";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";

type AppSidebarProps = {
  role: string;
  view: View;
  onNavigate: (view: View) => void;
};

const items: Array<{ view: View; labelKey: string; icon: typeof Gauge; adminOnly?: boolean }> = [
  { view: "dashboard", labelKey: "nav.dashboard", icon: Gauge },
  { view: "myKeys", labelKey: "nav.my_keys", icon: KeyRound },
  { view: "users", labelKey: "nav.users", icon: UserRound, adminOnly: true },
  { view: "accounts", labelKey: "nav.accounts", icon: Database, adminOnly: true },
  { view: "runtime", labelKey: "nav.runtime", icon: Activity },
];

export function AppSidebar({ role, view, onNavigate }: AppSidebarProps) {
  const { t } = useLocale();
  return (
    <Sidebar>
      <SidebarHeader>
        <div className="flex items-center gap-2 px-2 py-1.5">
          <div className="flex size-8 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 via-indigo-600 to-blue-600 text-white shadow-sm">
            <BrandLogo className="size-7" />
          </div>
          <div className="grid flex-1 text-left text-sm leading-tight">
            <span className="truncate font-semibold">BuzzHive</span>
            <span className="truncate text-xs text-muted-foreground">{t("app.subtitle")}</span>
          </div>
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>{t("common.admin")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="gap-1">
              {items.filter((item) => !item.adminOnly || role === "admin").map((item) => (
                <SidebarMenuItem key={item.view}>
                  <SidebarMenuButton
                    type="button"
                    isActive={view === item.view}
                    className="hover:bg-primary/10 hover:text-foreground dark:hover:bg-primary/25 dark:hover:text-white data-[active=true]:bg-indigo-600 data-[active=true]:text-white data-[active=true]:hover:bg-indigo-600 data-[active=true]:hover:text-white"
                    onClick={() => onNavigate(item.view)}
                  >
                    <item.icon />
                    <span>{t(item.labelKey)}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  );
}
