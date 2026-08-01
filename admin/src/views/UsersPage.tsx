import { useState } from "react";
import { ChevronRight, Loader2, RotateCcw } from "lucide-react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../components/ui/alert-dialog";
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
  onResetWeeklyQuotas: () => Promise<boolean>;
}) {
  const { t } = useLocale();
  const [resetOpen, setResetOpen] = useState(false);
  const [resetting, setResetting] = useState(false);

  async function resetWeeklyQuotas() {
    setResetting(true);
    try {
      if (await props.onResetWeeklyQuotas()) setResetOpen(false);
    } finally {
      setResetting(false);
    }
  }

  return (
    <div className="stack">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold tracking-tight">{t("nav.users")}</h2>
        <div className="flex items-center gap-2">
          <Button variant="outline" type="button" onClick={() => setResetOpen(true)}><RotateCcw size={15} /> {t("users.reset_weekly_quotas")}</Button>
          <Button type="button" onClick={props.onNewUser}>{t("users.new_user")}</Button>
        </div>
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
                <Button
                  variant="link"
                  className="h-auto p-0 font-medium dark:text-indigo-300 dark:hover:text-indigo-200"
                  type="button"
                  onClick={() => props.onOpenUser(user)}
                >
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
      <AlertDialog open={resetOpen} onOpenChange={setResetOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("users.reset_weekly_quotas_title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("users.reset_weekly_quotas_body")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction disabled={resetting} onClick={(event) => { event.preventDefault(); void resetWeeklyQuotas(); }}>
              {resetting && <Loader2 className="animate-spin" size={15} />}
              {t("users.reset_weekly_quotas")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
