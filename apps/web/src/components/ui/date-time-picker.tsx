"use client";
import * as React from "react";
import { ChevronDownIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Calendar } from "@/components/ui/calendar";
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
  const [open, setOpen] = React.useState(false);
  const timeInputId = React.useId();
  const date = value ? value.split("T")[0] : "";
  const time = value ? value.split("T")[1] || "" : "";
  const label = React.useMemo(() => {
    if (!value) return placeholder || "選択してください";
    const [d, t] = value.split("T");
    if (!d) return placeholder || "選択してください";
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
  }, [value, placeholder]);

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
      <PopoverContent className="w-[320px] max-w-[calc(100vw-2rem)] p-2" align="start">
        <div className="flex flex-col gap-3">
          <Calendar
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
              時刻
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
              クリア
            </Button>
            <Button type="button" onClick={() => setOpen(false)}>
              決定
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
