import { Alert, AlertDescription } from "../../components/ui/alert";
import { Table, TableBody, TableCell, TableRow } from "../../components/ui/table";
import { useLocale } from "../../i18n/locale";

export function UsageByKeyTable(props: { usage: Record<string, number> }) {
  const { t } = useLocale();
  const rows = Object.entries(props.usage).sort((a, b) => b[1] - a[1]);
  if (!rows.length) return <Alert><AlertDescription>{t("usage.no_usage")}</AlertDescription></Alert>;
  return (
    <Table>
      <TableBody>
        {rows.map(([key, count]) => (
          <TableRow key={key}>
            <TableCell className="mono">{key}</TableCell>
            <TableCell className="right mono">{count}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
