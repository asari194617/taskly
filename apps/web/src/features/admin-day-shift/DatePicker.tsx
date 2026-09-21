import * as React from "react";
import { Calendar as CalendarIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

type Props = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
};

export function DatePicker({ value, onChange, placeholder }: Props) {
  const [open, setOpen] = React.useState(false);
  const date = React.useMemo(() => {
    if (!value) return undefined;
    const [year, month, day] = value
      .split("-")
      .map((part) => Number.parseInt(part, 10));
    if (!year || !month || !day) return undefined;
    return new Date(year, month - 1, day);
  }, [value]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          data-empty={!date}
          className="data-[empty=true]:text-muted-foreground w-[220px] justify-start text-left font-normal"
          type="button"
        >
          <CalendarIcon className="mr-2 h-4 w-4" />
          {date ? date.toLocaleDateString() : <span>{placeholder || "日付を選択"}</span>}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0">
        <Calendar
          mode="single"
          selected={date}
          onSelect={(selected: Date | undefined) => {
            const next = selected
              ? `${selected.getFullYear()}-${String(selected.getMonth() + 1).padStart(2, "0")}-${String(selected.getDate()).padStart(2, "0")}`
              : "";
            onChange?.(next);
            setOpen(false);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
