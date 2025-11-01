"use client";
import * as React from "react";
import { ChevronDownIcon, Calendar as CalendarIcon, Clock } from "lucide-react";
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
};

// Minimal date-time picker built with native inputs inside a popover.
export function DateTimePicker({ value, onChange, placeholder }: Props) {
  const [open, setOpen] = React.useState(false);
  const date = value ? value.split("T")[0] : "";
  const time = value ? value.split("T")[1] || "" : "";
  const label = React.useMemo(() => {
    if (!value) return placeholder || "選択してください";
    const [d, t] = value.split("T");
    if (!d) return placeholder || "選択してください";
    const dt = new Date(`${d}${t ? "T" + t : "T00:00"}`);
    // Fallback to raw when Date is invalid
    return isNaN(dt.getTime()) ? value.replace("T", " ") : dt.toLocaleString();
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
        >
          <span className="truncate text-left">{label}</span>
          <ChevronDownIcon className="ml-2 size-4 shrink-0 opacity-60" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[280px] p-3" align="start">
        <div className="flex flex-col gap-3">
          {/* Nested popover for selecting date */}
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="w-full justify-start">
                <CalendarIcon className="mr-2 size-4 opacity-70" />
                <span>
                  {date
                    ? new Date(`${date}T00:00`).toLocaleDateString()
                    : "日付を選択"}
                </span>
              </Button>
            </PopoverTrigger>
            <PopoverContent className="p-3" align="start">
              <Calendar
                mode="single"
                selected={parseLocalDate(date)}
                captionLayout="dropdown"
                onSelect={(d) => commit(formatLocalDate(d), time)}
              />
            </PopoverContent>
          </Popover>

          {/* Nested popover for selecting time */}
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="w-full justify-start">
                <Clock className="mr-2 size-4 opacity-70" />
                <span>{time ? time : "時刻を選択"}</span>
              </Button>
            </PopoverTrigger>
            <PopoverContent className="p-3" align="start">
              <div>
                <div className="mb-1 text-xs text-muted-foreground">時刻</div>
                <Input
                  type="time"
                  step="1"
                  value={time}
                  onChange={(e) => commit(date, e.target.value)}
                  className="bg-background"
                />
              </div>
            </PopoverContent>
          </Popover>

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
