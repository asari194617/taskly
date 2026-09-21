import * as React from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import type { Subject } from "./api";

type Props = {
  value: string;
  onChange: (value: string) => void;
  subjects: Subject[];
  language: "ja" | "en";
  searchLabel: string;
  emptyLabel: string;
  placeholder: string;
};

export function SubjectSelect({
  value,
  onChange,
  subjects,
  language,
  searchLabel,
  emptyLabel,
  placeholder,
}: Props) {
  const [open, setOpen] = React.useState(false);
  const [filter, setFilter] = React.useState("");
  const displayName = React.useCallback(
    (subject: Subject) =>
      language === "en" && subject.name_en
        ? subject.name_en
        : subject.name_ja,
    [language],
  );
  const selected = subjects.find((subject) => String(subject.id) === value);
  const normalizedFilter = filter.toLocaleLowerCase();
  const filtered = subjects.filter((subject) =>
    displayName(subject).toLocaleLowerCase().includes(normalizedFilter),
  );

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          type="button"
          className="w-full justify-between"
        >
          <span className="truncate">
            {selected ? displayName(selected) : placeholder}
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="currentColor"
            className="h-4 w-4 opacity-60"
          >
            <path d="M12 15.5 6 9.5 7.5 8l4.5 4.5L16.5 8 18 9.5z" />
          </svg>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-[var(--radix-dropdown-menu-trigger-width)] p-2">
        <div className="p-1">
          <Input
            placeholder={searchLabel}
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
          />
        </div>
        <div className="max-h-60 overflow-auto">
          {filtered.map((subject) => (
            <DropdownMenuItem
              key={subject.id}
              onClick={() => {
                onChange(String(subject.id));
                setOpen(false);
              }}
            >
              {displayName(subject)}
            </DropdownMenuItem>
          ))}
          {filtered.length === 0 && (
            <div className="px-2 py-6 text-center text-sm text-muted-foreground">
              {emptyLabel}
            </div>
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
