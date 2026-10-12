"use client";
import * as React from "react";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { ChevronDown } from "lucide-react";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Calendar } from "@/components/ui/calendar";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export type DeadlineMeta =
  | { kind: "none" }
  | { kind: "outside"; slot: "朝" | "昼休み" | "放課後" }
  | { kind: "one"; period: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 }
  | { kind: "two"; pair: "1,2" | "3,4" | "5,6" | "7,8" | "9,10" }
  | {
      kind: "range";
      start: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10;
      end: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10;
    };

type Props = {
  id?: string;
  inline?: boolean;
  date: string;
  meta: DeadlineMeta | null;
  onChange: (date: string, meta: DeadlineMeta | null) => void;
};

const OUTSIDE_SLOTS = ["朝", "昼休み", "放課後"] as const;
const ONE_OPTIONS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] as const;
const TWO_OPTIONS = ["1,2", "3,4", "5,6", "7,8", "9,10"] as const;

export function DeadlinePicker({
  id,
  inline = false,
  date,
  meta,
  onChange,
}: Props) {
  const [open, setOpen] = React.useState(false);
  const [dateOpen, setDateOpen] = React.useState(false);
  const [tab, setTab] = React.useState<"outside" | "one" | "two" | "range">(
    meta?.kind === "outside"
      ? "outside"
      : meta?.kind === "one"
        ? "one"
        : meta?.kind === "two"
          ? "two"
          : meta?.kind === "range"
            ? "range"
            : "outside",
  );
  const [selectingRangeEnd, setSelectingRangeEnd] = React.useState(false);
  const [tmpDate, setTmpDate] = React.useState(date);
  const [outside, setOutside] = React.useState<"朝" | "昼休み" | "放課後">(
    meta?.kind === "outside" ? meta.slot : "朝",
  );
  const [one, setOne] = React.useState<1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10>(
    meta?.kind === "one" ? meta.period : 1,
  );
  const [two, setTwo] = React.useState<"1,2" | "3,4" | "5,6" | "7,8" | "9,10">(
    meta?.kind === "two" ? meta.pair : "1,2",
  );
  const [start, setStart] = React.useState<
    1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10
  >(meta?.kind === "range" ? meta.start : 1);
  const [end, setEnd] = React.useState<1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10>(
    meta?.kind === "range" ? meta.end : 2,
  );

  React.useEffect(() => {
    setSelectingRangeEnd(false);
    setTmpDate(date);
    setTab(meta?.kind && meta.kind !== "none" ? meta.kind : "outside");
    setOutside(meta?.kind === "outside" ? meta.slot : "朝");
    setOne(meta?.kind === "one" ? meta.period : 1);
    setTwo(meta?.kind === "two" ? meta.pair : "1,2");
    setStart(meta?.kind === "range" ? meta.start : 1);
    setEnd(meta?.kind === "range" ? meta.end : 2);
  }, [date, meta, open]);
  const enums = sinceUntilFromMeta(date, meta);
  const label = date
    ? `${date} ${deriveLabelFromEnums(date, enums.deadline_since, enums.deadline_until)}`.trim()
    : "未設定";

  function commit() {
    let m: DeadlineMeta | null = null;
    if (tmpDate) {
      if (tab === "outside") m = { kind: "outside", slot: outside };
      if (tab === "one") m = { kind: "one", period: one } as DeadlineMeta;
      if (tab === "two") m = { kind: "two", pair: two } as DeadlineMeta;
      if (tab === "range") m = { kind: "range", start, end } as DeadlineMeta;
    }
    onChange(tmpDate, m);
    setOpen(false);
  }

  const editor = (
    <div className="space-y-3">
      <div>
        {inline ? (
          <label className="block space-y-2">
            <span>日付</span>
            <input
              type="date"
              aria-label="日付"
              value={tmpDate}
              className="h-11 w-full min-w-0 rounded-md border bg-background px-3 text-base"
              onChange={(event) => setTmpDate(event.target.value)}
            />
          </label>
        ) : (
          <>
            <Label htmlFor="dp-date">日付</Label>
            <Button
              id="dp-date"
              type="button"
              variant="outline"
              aria-expanded={dateOpen}
              className="mt-2 w-48 justify-between font-normal"
              onClick={() => setDateOpen((current) => !current)}
            >
              {tmpDate || "日付を選択"}
              <ChevronDown className="h-4 w-4" />
            </Button>
            {dateOpen && (
              <div className="mt-2 w-fit rounded-md border bg-popover shadow-md">
                <Calendar
                  mode="single"
                  selected={tmpDate ? new Date(tmpDate) : undefined}
                  onSelect={(d: Date | undefined) => {
                    setTmpDate(
                      d
                        ? `${d.getFullYear()}-${String(
                            d.getMonth() + 1,
                          ).padStart(2, "0")}-${String(d.getDate()).padStart(
                            2,
                            "0",
                          )}`
                        : "",
                    );
                    setDateOpen(false);
                  }}
                />
              </div>
            )}
          </>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <button
          type="button"
          onClick={() => setTab("outside")}
          className={`min-h-11 px-2 py-1 rounded ${
            tab === "outside" ? "bg-accent text-accent-foreground" : ""
          }`}
        >
          授業外
        </button>
        <button
          type="button"
          onClick={() => setTab("one")}
          className={`min-h-11 px-2 py-1 rounded ${
            tab === "one" ? "bg-accent text-accent-foreground" : ""
          }`}
        >
          1コマ
        </button>
        <button
          type="button"
          onClick={() => setTab("two")}
          className={`min-h-11 px-2 py-1 rounded ${
            tab === "two" ? "bg-accent text-accent-foreground" : ""
          }`}
        >
          2コマ
        </button>
        <button
          type="button"
          onClick={() => setTab("range")}
          className={`min-h-11 px-2 py-1 rounded ${
            tab === "range" ? "bg-accent text-accent-foreground" : ""
          }`}
        >
          範囲選択
        </button>
      </div>
      {tab === "outside" && (
        <div>
          <Label>時間帯</Label>
          <div className="mt-1">
            <Select
              value={outside}
              onValueChange={(v: string) => setOutside(v as any)}
            >
              <SelectTrigger className={inline ? "min-h-11 w-full text-base" : undefined}>
                <SelectValue placeholder="選択" />
              </SelectTrigger>
              <SelectContent>
                {OUTSIDE_SLOTS.map((s) => (
                  <SelectItem key={s} value={s}>
                    {s}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
      {tab === "one" && (
        <div>
          <Label>コマ</Label>
          <div className="mt-1">
            <Select
              value={String(one)}
              onValueChange={(v: string) => setOne(parseInt(v, 10) as any)}
            >
              <SelectTrigger className={inline ? "min-h-11 w-full text-base" : undefined}>
                <SelectValue placeholder="選択" />
              </SelectTrigger>
              <SelectContent>
                {ONE_OPTIONS.map((s) => (
                  <SelectItem key={s} value={String(s)}>
                    {s}限
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
      {tab === "two" && (
        <div>
          <Label>コマ</Label>
          <div className="mt-1">
            <Select value={two} onValueChange={(v: string) => setTwo(v as any)}>
              <SelectTrigger className={inline ? "min-h-11 w-full text-base" : undefined}>
                <SelectValue placeholder="選択" />
              </SelectTrigger>
              <SelectContent>
                {TWO_OPTIONS.map((s) => (
                  <SelectItem key={s} value={s}>
                    {s}限
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
      {tab === "range" && (
        <div>
          <Label>範囲</Label>
          <div className="mt-2 grid grid-cols-2 sm:grid-cols-4 gap-2">
            {ONE_OPTIONS.map((s) => {
              const selected = s >= start && s <= end;
              return (
                <button
                  key={s}
                  type="button"
                  aria-pressed={selected}
                  className={
                    "h-11 rounded-md border text-sm px-2 " +
                    (selected
                      ? "bg-primary text-primary-foreground"
                      : "bg-background")
                  }
                  onClick={() => {
                    if (!selectingRangeEnd) {
                      setStart(s);
                      setEnd(s);
                      setSelectingRangeEnd(true);
                    } else {
                      setStart(Math.min(start, s) as typeof start);
                      setEnd(Math.max(start, s) as typeof end);
                      setSelectingRangeEnd(false);
                    }
                  }}
                >
                  {s}限
                </button>
              );
            })}
          </div>
          <div className="text-xs text-muted-foreground mt-1">
            クリックで開始、もう一度クリックまたは別のコマで終了を設定
          </div>
        </div>
      )}
      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="secondary"
          onClick={() => setOpen(false)}
        >
          閉じる
        </Button>
        <Button type="button" onClick={commit}>
          適用
        </Button>
      </div>
    </div>
  );
  const trigger = (
    <Button
      id={id}
      type="button"
      variant="outline"
      className="h-11 w-full justify-start text-base md:h-9 md:text-sm"
      aria-expanded={open}
      onClick={inline ? () => setOpen(!open) : undefined}
    >
      <span className="truncate">{label}</span>
    </Button>
  );
  if (inline)
    return (
      <div className="space-y-2">
        {trigger}
        {open && (
          <div className="rounded-xl border bg-muted/20 p-3">{editor}</div>
        )}
      </div>
    );
  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (!nextOpen) setDateOpen(false);
      }}
    >
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent className="w-96 max-w-[calc(100vw-2rem)] max-h-[var(--radix-popover-content-available-height)] overflow-y-auto">
        {editor}
      </PopoverContent>
    </Popover>
  );
}

// Utility to compute a representative RFC3339 moment if needed
// rfc3339FromMeta removed — server uses date + enums

// Compute deadline_since and deadline_until from date + meta, following the server’s mapping
export function sinceUntilFromMeta(
  date: string,
  meta: DeadlineMeta | null,
): { deadline_since?: string; deadline_until?: string } {
  if (!date || !meta) return {};
  // Map to enums: NONE|MORNING|P1..P6
  const toEnum = (period: number) => `P${period}`;
  switch (meta.kind) {
    case "outside": {
      if (meta.slot === "朝") {
        return { deadline_until: "MORNING" };
      }
      if (meta.slot === "昼休み") {
        return { deadline_until: "LUNCH" };
      }
      if (meta.slot === "放課後") {
        return { deadline_until: "AFTER_SCHOOL" };
      }
      return { deadline_since: "NONE", deadline_until: "NONE" };
    }
    case "one": {
      const p = Math.min(10, Math.max(1, meta.period));
      return { deadline_since: toEnum(p), deadline_until: toEnum(p) };
    }
    case "two": {
      const map: Record<string, [number, number]> = {
        "1,2": [1, 2],
        "3,4": [3, 4],
        "5,6": [5, 6],
        "7,8": [7, 8],
        "9,10": [9, 10],
      };
      const [s, e] = map[meta.pair];
      return { deadline_since: toEnum(s), deadline_until: toEnum(e) };
    }
    case "range": {
      const s = Math.min(10, Math.max(1, meta.start));
      const e = Math.min(10, Math.max(1, meta.end));
      const [ss, ee] = s <= e ? [s, e] : [e, s];
      return { deadline_since: toEnum(ss), deadline_until: toEnum(ee) };
    }
    default:
      return {};
  }
}

// Friendly label for UI from since/until
export function deriveLabelFromEnums(
  date: string,
  since?: string | null,
  until?: string | null,
  lang?: "ja" | "en",
): string {
  const l = lang || "ja";
  const norm = (s?: string | null) => (s || "").toUpperCase();
  const s = norm(since);
  const u = norm(until);
  if (u === "MORNING") return l === "ja" ? `朝まで` : `until morning`;
  if (u === "LUNCH") return l === "ja" ? `昼休みまで` : `until lunch`;
  if (u === "AFTER_SCHOOL")
    return l === "ja" ? `放課後まで` : `until after school`;
  const toNum = (v: string) =>
    v.startsWith("P") ? parseInt(v.slice(1), 10) : 0;
  const ps = toNum(s);
  const pu = toNum(u);
  if (pu && ps) {
    return ps === pu
      ? l === "ja"
        ? `${pu}限まで`
        : `until period ${pu}`
      : l === "ja"
        ? `${ps}〜${pu}限まで`
        : `period ${ps} to ${pu}`;
  }
  if (pu) return l === "ja" ? `${pu}限まで` : `until period ${pu}`;
  return "";
}
