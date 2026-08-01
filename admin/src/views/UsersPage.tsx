import { ChevronRight } from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
import { useLocale } from "../i18n/locale";
import type { AppUser } from "../types/admin";

export function UsersPage(props: {
  users: AppUser[];
  onNewUser: () => void;
  onOpenUser: (user: AppUser) => void;
}) {
  const { t } = useLocale();

  return (
    <div className="stack">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold tracking-tight">{t("nav.users")}</h2>
        <Button type="button" onClick={props.onNewUser}>{t("users.new_user")}</Button>
      </div>
      <div className="data-table-card">
        <Table className="keys-table-inner">
          <TableHeader>
            <TableRow>
              <TableHead>{t("auth.username")}</TableHead>
              <TableHead>{t("users.role")}</TableHead>
              <TableHead>{t("common.status")}</TableHead>
              <TableHead className="right">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{props.users.map((user) => (
            <TableRow key={user.id}>
              <TableCell>
                <Button variant="link" className="h-auto p-0 font-medium" type="button" onClick={() => props.onOpenUser(user)}>
                  {user.username}
                </Button>
              </TableCell>
              <TableCell>{user.role}</TableCell>
              <TableCell>
                {user.valid ? (
                  <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>
                ) : (
                  <Badge variant="secondary">{t("common.disabled")}</Badge>
                )}
              </TableCell>
              <TableCell className="right">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="icon-sm" type="button" aria-label={t("users.open_detail")} onClick={() => props.onOpenUser(user)}>
                      <ChevronRight size={15} />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t("users.open_detail")}</TooltipContent>
                </Tooltip>
              </TableCell>
            </TableRow>
          ))}</TableBody>
        </Table>
      </div>
    </div>
  );
}
