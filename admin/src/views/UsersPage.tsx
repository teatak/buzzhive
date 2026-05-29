import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { useLocale } from "../i18n/locale";
import type { AppUser } from "../types/admin";

export function UsersPage(props: {
  users: AppUser[];
  onNewUser: () => void;
}) {
  const { t } = useLocale();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("nav.users")}</CardTitle>
        <CardAction>
          <Button type="button" onClick={props.onNewUser}>{t("users.new_user")}</Button>
        </CardAction>
      </CardHeader>
      <CardContent className="min-w-0 overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("auth.username")}</TableHead>
              <TableHead>{t("users.role")}</TableHead>
              <TableHead>{t("common.status")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{props.users.map((user) => (
            <TableRow key={user.id}>
              <TableCell>{user.username}</TableCell>
              <TableCell>{user.role}</TableCell>
              <TableCell>
                {user.valid ? (
                  <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>
                ) : (
                  <Badge variant="secondary">{t("common.disabled")}</Badge>
                )}
              </TableCell>
            </TableRow>
          ))}</TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
