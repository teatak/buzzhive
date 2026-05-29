import type { ReactNode } from "react";
import { Trash2 } from "lucide-react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Checkbox } from "../components/ui/checkbox";
import { Input } from "../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Textarea } from "../components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
import { useLocale } from "../i18n/locale";
import type { AdminKey, GoogleAccount } from "../types/admin";

export function AccountsPage(props: {
  accounts: GoogleAccount[];
  keys: AdminKey[];
  byKey: Record<string, number>;
  newAccount: { email: string };
  newKey: { account_id: string; key: string };
  keyAccountFilter: string;
  selectedKeyIds: number[];
  filteredKeys: AdminKey[];
  allFilteredSelected: boolean;
  keyCountByAccount: Record<number, number>;
  keyStatus: (key: AdminKey) => ReactNode;
  onNewAccountChange: (value: { email: string }) => void;
  onNewKeyChange: (value: { account_id: string; key: string }) => void;
  onKeyAccountFilterChange: (value: string) => void;
  onCreateAccount: () => void;
  onCreateKey: () => void;
  onFlushCooling: () => void;
  onUpdateGoogleAccount: (account: GoogleAccount, enabled: boolean) => void;
  onRequestDeleteGoogleAccount: (account: GoogleAccount) => void;
  onRequestDeleteAPIKeys: (ids: number[]) => void;
  onToggleKeySelection: (id: number, checked: boolean) => void;
  onToggleAllFilteredKeys: (checked: boolean) => void;
}) {
  const { t } = useLocale();

  return (
    <div className="stack">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle>{t("nav.accounts")}</CardTitle>
            <Badge variant="secondary">{props.accounts.length} {t("accounts.account")}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <div className="form-row account-form">
            <Input placeholder={t("accounts.email")} value={props.newAccount.email} onChange={(event) => props.onNewAccountChange({ ...props.newAccount, email: event.target.value })} />
            <Button type="button" onClick={props.onCreateAccount} disabled={!props.newAccount.email}>{t("common.add")}</Button>
          </div>
          <Table className="section-table">
            <TableHeader><TableRow><TableHead>{t("accounts.email")}</TableHead><TableHead>{t("accounts.prefix")}</TableHead><TableHead>{t("common.status")}</TableHead><TableHead className="right">{t("accounts.keys")}</TableHead><TableHead /></TableRow></TableHeader>
            <TableBody>{props.accounts.map((account) => (
              <TableRow key={account.id}>
                <TableCell>{account.email}</TableCell>
                <TableCell className="mono">{account.prefix}</TableCell>
                <TableCell>{account.enabled ? <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge> : <Badge variant="secondary">{t("common.disabled")}</Badge>}</TableCell>
                <TableCell className="right mono">{props.keyCountByAccount[account.id] ?? 0}</TableCell>
                <TableCell className="right">
                  <Button variant="outline" type="button" onClick={() => props.onUpdateGoogleAccount(account, !account.enabled)}>
                    {account.enabled ? t("common.disable") : t("common.enable")}
                  </Button>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        type="button"
                        aria-label={t("common.delete")}
                        onClick={() => props.onRequestDeleteGoogleAccount(account)}
                      >
                        <Trash2 size={15} />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t("common.delete")}</TooltipContent>
                  </Tooltip>
                </TableCell>
              </TableRow>
            ))}</TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("keys.gemini_keys")}</CardTitle>
          <CardAction>
            <Button variant="outline" type="button" onClick={props.onFlushCooling}>
              <Trash2 size={16} /> {t("keys.clear_cooling")}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <div className="api-key-form">
            <Select
              value={props.newKey.account_id || "none"}
              onValueChange={(value) => props.onNewKeyChange({ ...props.newKey, account_id: value === "none" ? "" : value })}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{t("keys.google_account")}</SelectItem>
                {props.accounts.map((account) => <SelectItem key={account.id} value={String(account.id)}>{account.email}</SelectItem>)}
              </SelectContent>
            </Select>
            <Textarea
              placeholder={"AIza...\nAIza..."}
              rows={4}
              value={props.newKey.key}
              onChange={(event) => props.onNewKeyChange({ ...props.newKey, key: event.target.value })}
            />
            <Button type="button" onClick={props.onCreateKey} disabled={!props.newKey.account_id || !props.newKey.key.trim()}>{t("common.add")}</Button>
          </div>
          <div className="filter-row">
            <Select value={props.keyAccountFilter} onValueChange={props.onKeyAccountFilterChange}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("accounts.all")}</SelectItem>
                {props.accounts.map((account) => <SelectItem key={account.id} value={String(account.id)}>{account.email}</SelectItem>)}
              </SelectContent>
            </Select>
            <Button
              className="justify-self-end"
              variant="outline"
              size="sm"
              type="button"
              disabled={!props.selectedKeyIds.length}
              onClick={() => props.onRequestDeleteAPIKeys(props.selectedKeyIds)}
            >
              <Trash2 size={16} /> {t("keys.delete_selected")}
            </Button>
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead><Checkbox checked={props.allFilteredSelected} onCheckedChange={(checked) => props.onToggleAllFilteredKeys(checked === true)} /></TableHead>
                <TableHead>{t("keys.name")}</TableHead>
                <TableHead>{t("accounts.account")}</TableHead>
                <TableHead>{t("keys.key")}</TableHead>
                <TableHead>{t("common.status")}</TableHead>
                <TableHead className="right">{t("keys.requests")}</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>{props.filteredKeys.map((key) => (
              <TableRow key={key.name}>
                <TableCell><Checkbox checked={props.selectedKeyIds.includes(key.id)} onCheckedChange={(checked) => props.onToggleKeySelection(key.id, checked === true)} /></TableCell>
                <TableCell className="mono">{key.name}</TableCell>
                <TableCell>{key.account_email || <span className="muted">{t("keys.unmapped")}</span>}</TableCell>
                <TableCell className="mono">{key.key}</TableCell>
                <TableCell>{props.keyStatus(key)}</TableCell>
                <TableCell className="right mono">{props.byKey[key.name] ?? 0}</TableCell>
                <TableCell className="right">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        type="button"
                        aria-label={t("common.delete")}
                        onClick={() => props.onRequestDeleteAPIKeys([key.id])}
                      >
                        <Trash2 size={15} />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t("common.delete")}</TooltipContent>
                  </Tooltip>
                </TableCell>
              </TableRow>
            ))}</TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
