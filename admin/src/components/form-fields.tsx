import { useId, type FocusEvent, type KeyboardEvent, type ReactNode } from "react";
import { CircleHelp } from "lucide-react";
import { Field, FieldLabel } from "./ui/field";
import { Input } from "./ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";
import { Textarea } from "./ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";
import { cn } from "../lib/utils";

type SelectOption = {
  value: string;
  label: string;
};

function LabelWithTip(props: { htmlFor?: string; label: string; tip?: string }) {
  return (
    <FieldLabel htmlFor={props.htmlFor} className="inline-flex items-center gap-1.5">
      {props.label}
      {props.tip && (
        <Tooltip>
          <TooltipTrigger asChild>
            <button type="button" tabIndex={-1} className="inline-flex text-muted-foreground hover:text-foreground" aria-label={props.tip}>
              <CircleHelp className="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent className="max-w-80 text-sm leading-relaxed">
            {props.tip}
          </TooltipContent>
        </Tooltip>
      )}
    </FieldLabel>
  );
}

export function FormTextField(props: {
  label: string;
  tip?: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  onBlur?: (event: FocusEvent<HTMLInputElement>) => void;
  onKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void;
}) {
  const id = useId();

  return (
    <Field>
      <LabelWithTip htmlFor={id} label={props.label} tip={props.tip} />
      <Input
        id={id}
        type={props.type}
        value={props.value}
        onBlur={props.onBlur}
        onKeyDown={props.onKeyDown}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </Field>
  );
}

export function FormNumberField(props: { label: string; tip?: string; value: number; onChange: (value: number) => void }) {
  const id = useId();

  return (
    <Field>
      <LabelWithTip htmlFor={id} label={props.label} tip={props.tip} />
      <Input id={id} type="number" value={props.value} onChange={(event) => props.onChange(Number(event.target.value))} />
    </Field>
  );
}

export function FormSelectField(props: { label: string; tip?: string; value: string; options: SelectOption[]; onChange: (value: string) => void }) {
  return (
    <Field>
      <LabelWithTip label={props.label} tip={props.tip} />
      <Select value={props.value} onValueChange={props.onChange}>
        <SelectTrigger><SelectValue /></SelectTrigger>
        <SelectContent>
          {props.options.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
        </SelectContent>
      </Select>
    </Field>
  );
}

export function FormTextareaField(props: { label: string; tip?: string; value: string; onChange: (value: string) => void; className?: string }) {
  const id = useId();

  return (
    <Field className="min-w-0">
      <LabelWithTip htmlFor={id} label={props.label} tip={props.tip} />
      <Textarea id={id} className={cn("min-w-0 max-w-full", props.className)} value={props.value} onChange={(event) => props.onChange(event.target.value)} />
    </Field>
  );
}

export function FormStaticField(props: { label: string; tip?: string; children: ReactNode }) {
  return (
    <Field>
      <LabelWithTip label={props.label} tip={props.tip} />
      {props.children}
    </Field>
  );
}
