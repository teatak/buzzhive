import { Check, Copy, Eye, EyeOff, KeyRound, Trash2 } from "lucide-react";
import { Alert, AlertDescription } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "../components/ui/tooltip";
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
  onCopyText: (value: string, target: string) => void;
  onCopyUserAPIKey: (key: UserAPIKey) => void;
  onToggleUserAPIKeyReveal: (key: UserAPIKey) => void;
  onUpdateUserAPIKey: (key: UserAPIKey, valid: boolean) => void;
  onRequestDeleteUserAPIKey: (key: UserAPIKey) => void;
}) {
  const { t } = useLocale();

  return (
    <Card className="keys-card">
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>{t("nav.my_keys")}</CardTitle>
          <Badge variant="secondary">{props.userAPIKeys.length} {t("accounts.keys")}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="key-create-panel">
          <Input
            className="key-create-input"
            placeholder={t("keys.key_name")}
            value={props.newUserKey.name}
            onChange={(event) => props.onNewUserKeyChange({ ...props.newUserKey, name: event.target.value })}
          />
          <Button type="button" onClick={props.onCreateUserAPIKey}><KeyRound size={15} /> {t("keys.generate")}</Button>
        </div>
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
        <div className="keys-table-scroll">
          <div className="keys-table-inner">
            <Table className="keys-table">
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
                      <Button size="sm" variant="outline" type="button" onClick={() => props.onUpdateUserAPIKey(key, !key.valid)}>
                        {key.valid ? t("common.disable") : t("common.enable")}
                      </Button>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon-sm"
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
      </CardContent>
    </Card>
  );
}
