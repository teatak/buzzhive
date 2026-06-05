import { useState } from "react";
import { Check, Copy, Eye, EyeOff, KeyRound, Trash2 } from "lucide-react";
import { Alert, AlertDescription } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
import { EnabledToggleButton } from "../components/enabled-toggle-button";
import { useLocale } from "../i18n/locale";
import type { UserAPIKey } from "../types/admin";

export function MyApiKeysPage(props: {
  userAPIKeys: UserAPIKey[];
  newUserKey: { name: string; token: string };
  generatedUserKey: UserAPIKey | null;
  revealedUserKeys: Record<number, string>;
  copiedTarget: string;
  onNewUserKeyChange: (value: { name: string; token: string }) => void;
  onCreateUserAPIKey: () => void;
  onClearGeneratedUserKey: () => void;
  onCopyText: (value: string, target: string) => void;
  onCopyUserAPIKey: (key: UserAPIKey) => void;
  onToggleUserAPIKeyReveal: (key: UserAPIKey) => void;
  onUpdateUserAPIKey: (key: UserAPIKey, valid: boolean) => void;
  onRequestDeleteUserAPIKey: (key: UserAPIKey) => void;
}) {
  const { t } = useLocale();
  const [createOpen, setCreateOpen] = useState(false);

  function setCreateDialogOpen(open: boolean) {
    setCreateOpen(open);
    if (open) {
      props.onNewUserKeyChange({ name: "", token: "" });
      props.onClearGeneratedUserKey();
    }
  }

  return (
    <div className="stack">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <h2 className="text-2xl font-semibold tracking-tight">{t("nav.my_keys")}</h2>
          <Badge variant="secondary">{props.userAPIKeys.length} {t("keys.count")}</Badge>
        </div>
        <Button type="button" onClick={() => setCreateDialogOpen(true)}><KeyRound size={15} /> {t("keys.generate")}</Button>
      </div>
      <Dialog open={createOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("keys.generate")}</DialogTitle>
          </DialogHeader>
          <Input
            placeholder={t("keys.key_name")}
            value={props.newUserKey.name}
            onChange={(event) => props.onNewUserKeyChange({ ...props.newUserKey, name: event.target.value })}
          />
          {props.generatedUserKey && (
            <Alert className="generated-key-alert">
              <AlertDescription className="mono [overflow-wrap:anywhere]">{props.generatedUserKey.token}</AlertDescription>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    type="button"
                    aria-label={t("common.copy")}
                    onClick={() => props.onCopyText(props.generatedUserKey!.token, "generated-user-key")}
                  >
                    {props.copiedTarget === "generated-user-key" ? <Check className="text-emerald-600 dark:text-emerald-400" size={15} /> : <Copy size={15} />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("common.copy")}</TooltipContent>
              </Tooltip>
            </Alert>
          )}
          <DialogFooter>
            <Button variant="outline" type="button" onClick={() => setCreateDialogOpen(false)}>{t("common.cancel")}</Button>
            <Button type="button" disabled={!props.newUserKey.name.trim()} onClick={props.onCreateUserAPIKey}><KeyRound size={15} /> {t("keys.generate")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <div className="data-table-card">
            <Table className="keys-table keys-table-inner">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("keys.name")}</TableHead>
                  <TableHead>{t("keys.api_key")}</TableHead>
                  <TableHead>{t("common.status")}</TableHead>
                  <TableHead className="right">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>{props.userAPIKeys.map((key) => (
                <TableRow key={key.id}>
                  <TableCell><div className="key-name-cell"><KeyRound size={15} /> {key.name}</div></TableCell>
                  <TableCell>
                    <div className="key-token-cell">
                      <div className="key-token mono">
                        <span>{props.revealedUserKeys[key.id] ?? key.token}</span>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button className="key-token-copy" variant="ghost" size="icon-sm" type="button" aria-label={t("common.copy")} onClick={() => props.onCopyUserAPIKey(key)}>
                              {props.copiedTarget === `user-key-${key.id}` ? <Check className="text-emerald-600 dark:text-emerald-400" size={15} /> : <Copy size={15} />}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>{t("common.copy")}</TooltipContent>
                        </Tooltip>
                      </div>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="ghost" size="icon-sm" type="button" aria-label={props.revealedUserKeys[key.id] ? t("common.hide") : t("common.show")} onClick={() => props.onToggleUserAPIKeyReveal(key)}>
                            {props.revealedUserKeys[key.id] ? <EyeOff size={15} /> : <Eye size={15} />}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{props.revealedUserKeys[key.id] ? t("common.hide") : t("common.show")}</TooltipContent>
                      </Tooltip>
                    </div>
                  </TableCell>
                  <TableCell>
                    {key.valid ? (
                      <Badge variant="outline" className="border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">{t("common.active")}</Badge>
                    ) : (
                      <Badge variant="secondary">{t("common.disabled")}</Badge>
                    )}
                  </TableCell>
                  <TableCell className="right">
                    <div className="key-actions">
                      <EnabledToggleButton enabled={key.valid} onClick={() => props.onUpdateUserAPIKey(key, !key.valid)} />
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                            type="button"
                            aria-label={t("common.delete")}
                            onClick={() => props.onRequestDeleteUserAPIKey(key)}
                          >
                            <Trash2 size={15} />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t("common.delete")}</TooltipContent>
                      </Tooltip>
                    </div>
                  </TableCell>
                </TableRow>
              ))}</TableBody>
            </Table>
      </div>
    </div>
  );
}
