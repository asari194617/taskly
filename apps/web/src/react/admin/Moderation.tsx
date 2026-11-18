"use client";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useToast } from "@/hooks/use-toast";
import { getApiBase, webFetch } from "@/lib/api";

type FlaggedRow = {
  id: number;
  task_id: number;
  task_title: string;
  category: string;
  severity: string;
  task_hidden: boolean;
};

type Policy = {
  default_auto_hide: "low" | "medium" | "high";
  auto_hide_by_category?: Record<string, string>;
};

async function api<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const response = await webFetch(`${getApiBase()}${path}`, opts);
  if (!response.ok) throw new Error((await response.text()) || response.statusText);
  const body = await response.text();
  return (body ? JSON.parse(body) : undefined) as T;
}

export default function Moderation() {
  const [items, setItems] = React.useState<FlaggedRow[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [loadError, setLoadError] = React.useState("");
  const [busyTaskId, setBusyTaskId] = React.useState<number | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [policyLoaded, setPolicyLoaded] = React.useState(false);
  const [policy, setPolicy] = React.useState<Policy>({ default_auto_hide: "high" });
  const [overridesText, setOverridesText] = React.useState("{}");
  const [showHidden, setShowHidden] = React.useState(true);
  const { toast } = useToast();

  const load = React.useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      const [rows, settings] = await Promise.all([
        api<FlaggedRow[]>("/admin/moderation/flagged?limit=50"),
        api<Policy>("/admin/moderation/settings"),
      ]);
      setItems(rows || []);
      setPolicy(settings);
      setOverridesText(JSON.stringify(settings.auto_hide_by_category || {}, null, 2));
      setPolicyLoaded(true);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "読み込みに失敗しました");
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => { void load(); }, [load]);

  async function updateVisibility(row: FlaggedRow) {
    setBusyTaskId(row.task_id);
    try {
      await api<unknown>(`/admin/moderation/tasks/${row.task_id}/${row.task_hidden ? "unhide" : "hide"}`, { method: "POST" });
      setItems((previous) => previous.map((item) => item.task_id === row.task_id
        ? { ...item, task_hidden: !row.task_hidden }
        : item));
      toast({ title: row.task_hidden ? "課題を再表示しました" : "課題を非表示にしました" });
    } catch (error) {
      toast({ title: "変更に失敗しました", description: error instanceof Error ? error.message : undefined, variant: "destructive" });
    } finally {
      setBusyTaskId(null);
    }
  }

  async function savePolicy(event: React.FormEvent) {
    event.preventDefault();
    if (!policyLoaded) return;
    let overrides: Record<string, string>;
    try {
      const parsed: unknown = JSON.parse(overridesText);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed) ||
          Object.values(parsed).some((value) => !["low", "medium", "high"].includes(String(value)))) {
        throw new Error("カテゴリと重大度の組み合わせをJSONオブジェクトで入力してください");
      }
      overrides = parsed as Record<string, string>;
    } catch (error) {
      toast({ title: "カテゴリ別設定を確認してください", description: error instanceof Error ? error.message : undefined, variant: "destructive" });
      return;
    }
    setSaving(true);
    try {
      await api<unknown>("/admin/moderation/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...policy, auto_hide_by_category: overrides }),
      });
      setPolicy((previous) => ({ ...previous, auto_hide_by_category: overrides }));
      toast({ title: "自動非表示の設定を保存しました" });
    } catch (error) {
      toast({ title: "保存に失敗しました", description: error instanceof Error ? error.message : undefined, variant: "destructive" });
    } finally {
      setSaving(false);
    }
  }

  const visibleItems = showHidden ? items : items.filter((item) => !item.task_hidden);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">モデレーション</h1>
        <p className="mt-1 text-sm text-muted-foreground">フラグ付き課題の公開状態と自動非表示の基準を管理します。</p>
      </div>

      <Card className="p-4 sm:p-6">
        <form onSubmit={savePolicy} className="space-y-4">
          <div>
            <h2 className="font-semibold">自動非表示の設定</h2>
            <p className="mt-1 text-sm text-muted-foreground">指定した重大度以上の課題を自動で非表示にします。</p>
          </div>
          <div className="max-w-xs space-y-2">
            <Label htmlFor="default-auto-hide">標準の重大度</Label>
            <select id="default-auto-hide" value={policy.default_auto_hide}
              onChange={(event) => setPolicy((previous) => ({ ...previous, default_auto_hide: event.target.value as Policy["default_auto_hide"] }))}
              className="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
              <option value="low">低以上</option>
              <option value="medium">中以上</option>
              <option value="high">高のみ</option>
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="category-overrides">カテゴリ別の設定 (JSON)</Label>
            <textarea id="category-overrides" value={overridesText} onChange={(event) => setOverridesText(event.target.value)}
              spellCheck={false} rows={4} className="w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
              aria-describedby="category-overrides-help" />
            <p id="category-overrides-help" className="text-xs text-muted-foreground">例: {`{"threat":"medium"}`}。重大度は low、medium、high を指定します。</p>
          </div>
          <Button type="submit" disabled={saving || !policyLoaded}>{saving ? "保存中…" : "設定を保存"}</Button>
        </form>
      </Card>

      <section className="space-y-3" aria-labelledby="flagged-heading">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 id="flagged-heading" className="font-semibold">フラグ付き課題</h2>
            <p className="text-sm text-muted-foreground">{items.length} 件を表示</p>
          </div>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2 text-sm"><Input type="checkbox" checked={showHidden} onChange={(event) => setShowHidden(event.target.checked)} className="h-4 w-4" />非表示も表示</label>
            <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>再読み込み</Button>
          </div>
        </div>
        {loadError && <div role="alert" className="rounded-md border border-destructive p-4 text-sm text-destructive">{loadError}</div>}
        {loading ? <p className="text-sm text-muted-foreground">読み込み中…</p> : visibleItems.length === 0 ?
          <p className="rounded-lg border p-6 text-sm text-muted-foreground">該当する課題はありません。</p> :
          <div className="space-y-2">{visibleItems.map((row) => (
            <Card key={row.id} className="flex flex-wrap items-center justify-between gap-4 p-4">
              <div className="min-w-0 space-y-1">
                <p className="font-medium break-words">{row.task_title || `課題 ${row.task_id}`}</p>
                <p className="text-xs text-muted-foreground">課題 ID: {row.task_id} · {row.category} · 重大度: {row.severity}</p>
                <span className={row.task_hidden ? "text-xs font-medium text-destructive" : "text-xs font-medium text-muted-foreground"}>
                  {row.task_hidden ? "非表示" : "公開中"}
                </span>
              </div>
              <Button variant={row.task_hidden ? "outline" : "destructive"} size="sm"
                disabled={busyTaskId === row.task_id} onClick={() => void updateVisibility(row)}>
                {busyTaskId === row.task_id ? "変更中…" : row.task_hidden ? "再表示" : "非表示にする"}
              </Button>
            </Card>
          ))}</div>}
      </section>
    </div>
  );
}
