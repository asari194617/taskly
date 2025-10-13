"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";

export default function AdminCalendar() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [start, setStart] = React.useState("");
  const [end, setEnd] = React.useState("");
  const [isBiz, setIsBiz] = React.useState(true);
  const [notes, setNotes] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [msg, setMsg] = React.useState<string | null>(null);

  async function apply() {
    if (!token) return;
    try {
      setBusy(true);
      setMsg(null);
      await gql<{ adminCalendarUpsertRange: boolean }>(
        `mutation($s:String!,$e:String!,$b:Boolean!,$n:String){ adminCalendarUpsertRange(start:$s,end:$e,is_business_day:$b,notes:$n) }`,
        { s: start, e: end, b: isBiz, n: notes || undefined }
      );
      setMsg("反映しました");
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">営業日カレンダー</h1>
      <Card className="p-4 space-y-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <div className="text-sm mb-1">開始日</div>
            <Input
              type="date"
              value={start}
              onChange={(e) => setStart(e.target.value)}
            />
          </div>
          <div>
            <div className="text-sm mb-1">終了日</div>
            <Input
              type="date"
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Switch
            checked={isBiz}
            onCheckedChange={(v: boolean) => setIsBiz(v)}
          />
          <span className="text-sm">営業日として設定</span>
        </div>
        <div>
          <div className="text-sm mb-1">メモ（任意）</div>
          <Input
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="例: 祝日"
          />
        </div>
        {msg ? (
          <div className="text-sm text-muted-foreground">{msg}</div>
        ) : null}
        <div className="flex justify-end">
          <Button onClick={apply} disabled={!start || !end || busy}>
            {busy ? "適用中…" : "適用"}
          </Button>
        </div>
      </Card>
    </div>
  );
}
