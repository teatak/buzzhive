import { SidebarRail } from "@/components/ui/sidebar";

export function AppSidebarRail() {
  return (
    <SidebarRail className="z-10 !cursor-w-resize [[data-side=left][data-state=collapsed]_&]:!cursor-e-resize [[data-side=right][data-state=collapsed]_&]:!cursor-w-resize" />
  );
}
