"use client";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  const label = value
    ? value.replace("T", " ")
    : placeholder || "選択してください";

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
          className="w-full justify-start"
        >
          <span className="truncate">{label}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80">
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-2">
            <div>
              <div className="text-xs text-muted-foreground mb-1">日付</div>
              <Input
                type="date"
                value={date}
                onChange={(e) => commit(e.target.value, time)}
              />
            </div>
            <div>
              <div className="text-xs text-muted-foreground mb-1">時刻</div>
              <Input
                type="time"
                value={time}
                onChange={(e) => commit(date, e.target.value)}
              />
            </div>
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
