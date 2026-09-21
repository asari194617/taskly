"use client";
import * as React from "react";
import { ChevronDownIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Calendar } from "@/components/ui/calendar";
import { enUS, ja } from "date-fns/locale";
import { useLang } from "@/react/LanguageProvider";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

type Props = {
  value: string; // yyyy-MM-ddTHH:mm (local)
  onChange: (v: string) => void;
  placeholder?: string;
  autoFocus?: boolean;
};

// Keep the calendar and time input in one popover so selecting either does not
// dismiss the picker through a nested portal.
export function DateTimePicker({
  value,
  onChange,
  placeholder,
  autoFocus,
}: Props) {
  const { dict, lang } = useLang();
  const [open, setOpen] = React.useState(false);
  const timeInputId = React.useId();
  const date = value ? value.split("T")[0] : "";
  const time = value ? value.split("T")[1] || "" : "";
  const label = React.useMemo(() => {
    if (!value) return placeholder || dict.dateTimePicker.select;
    const [d, t] = value.split("T");
    if (!d) return placeholder || dict.dateTimePicker.select;
    const [yy, mm, dd] = d.split("-");
    const dateStr =
      yy && mm && dd ? `${yy}/${mm}/${dd}` : d.split("-").join("/");
    if (!t) return dateStr;
    const hhmmss = t.split(":");
    const hh = (hhmmss[0] || "00").padStart(2, "0");
    const mi = (hhmmss[1] || "00").padStart(2, "0");
    const ss = (hhmmss[2] || "").slice(0, 2);
    const timeStr = ss ? `${hh}:${mi}:${ss}` : `${hh}:${mi}`;
    return `${dateStr} ${timeStr}`;
  }, [value, placeholder, dict.dateTimePicker.select]);

  function parseLocalDate(d: string): Date | undefined {
    if (!d) return undefined;
    const [y, m, day] = d.split("-").map(Number);
    if (!y || !m || !day) return undefined;
    const date = new Date(y, m - 1, day);
    return isNaN(date.getTime()) ? undefined : date;
  }

  function formatLocalDate(d: Date | undefined): string {
    if (!d) return "";
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    return `${y}-${m}-${day}`;
  }

  function commit(nextDate: string, nextTime: string) {
    const v =
      nextDate && nextTime
        ? `${nextDate}T${nextTime}`
        : nextDate
        ? `${nextDate}T00:00`
        : "";
    onChange(v);
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          className="w-full justify-between font-normal"
          autoFocus={autoFocus}
        >
          <span className="truncate text-left">{label}</span>
          <ChevronDownIcon className="ml-2 size-4 shrink-0 opacity-60" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-[320px] max-w-[calc(100vw-2rem)] p-2"
        align="start"
        portalled={false}
      >
        <div className="flex flex-col gap-3">
          <Calendar
            locale={lang === "ja" ? ja : enUS}
            className="mx-auto p-0"
            classNames={{ week: "mt-1 flex w-full" }}
            mode="single"
            selected={parseLocalDate(date)}
            captionLayout="dropdown"
            startMonth={new Date(1970, 0)}
            endMonth={new Date(2100, 11)}
            onSelect={(d) => commit(formatLocalDate(d), time)}
          />
          <div>
            <label
              className="mb-1 block text-xs text-muted-foreground"
              htmlFor={timeInputId}
            >
              {dict.dateTimePicker.time}
            </label>
            <Input
              id={timeInputId}
              type="time"
              step="1"
              value={time}
              disabled={!date}
              onChange={(e) => commit(date, e.target.value)}
              className="bg-background"
            />
          </div>

          <div className="flex justify-between">
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                onChange("");
                setOpen(false);
              }}
            >
              {dict.dateTimePicker.clear}
            </Button>
            <Button type="button" onClick={() => setOpen(false)}>
              {dict.dateTimePicker.done}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
