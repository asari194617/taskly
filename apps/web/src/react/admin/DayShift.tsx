"use client";
import * as React from "react";
import { ArrowDownIcon, ArrowUpIcon, PlusIcon, UsersIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import * as api from "@/features/admin-day-shift/api";

const label = (u: api.AdminUser) => u.display_name?.trim() || u.email;
const selectClass = "h-10 w-full rounded-md border bg-background px-3 text-sm";

export default function AdminDayShift() {
  const [cycles, setCycles] = React.useState<api.Cycle[]>([]);
  const [cid, setCid] = React.useState(0);
  const [currentCid, setCurrentCid] = React.useState(0);
  const [index, setIndex] = React.useState(0);
  const [indexDraft, setIndexDraft] = React.useState("0");
  const [groups, setGroups] = React.useState<api.Group[]>([]);
  const [users, setUsers] = React.useState<Record<number, api.AdminUser>>({});
  const [loading, setLoading] = React.useState(true);
  const [loadFailed, setLoadFailed] = React.useState(false);
  const [groupsFailed, setGroupsFailed] = React.useState(false);
  const [groupsLoading, setGroupsLoading] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [revision, refresh] = React.useReducer((v) => v + 1, 0);
  const [dialog, setDialog] = React.useState<"cycle" | "group" | "csv" | null>(
    null,
  );
  const [editingCycle, setEditingCycle] = React.useState<api.Cycle | null>(
    null,
  );
  const [editing, setEditing] = React.useState<api.Group | null>(null);
  const [selected, setSelected] = React.useState<api.AdminUser[]>([]);
  const [query, setQuery] = React.useState("");
  const [options, setOptions] = React.useState<api.AdminUser[]>([]);
  const [searching, setSearching] = React.useState(false);
  const [searchError, setSearchError] = React.useState("");
  const [begin, setBegin] = React.useState("");
  const [finish, setFinish] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [csv, setCsv] = React.useState("");
  const [confirmation, setConfirmation] = React.useState<{
    title: string;
    description: string;
    action: () => Promise<unknown>;
  } | null>(null);

  React.useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadFailed(false);
    (async () => {
      const all: api.Cycle[] = [];
      for (let page = 1; ; page++) {
        const result = (await api.loadCycles(page, 100))
          .adminDayshiftCyclesPaginated;
        all.push(...result.items);
        if (all.length >= result.total || !result.items.length) break;
      }
      const [current, nextIndex] = await Promise.all([
        api.loadCurrentCycleId(),
        api.loadDayShiftIndex(),
      ]);
      if (!active) return;
      setCycles(all);
      setCurrentCid(current);
      setIndex(nextIndex);
      setIndexDraft(String(nextIndex));
      setCid((old) =>
        all.some((c) => c.id === old)
          ? old
          : all.find((c) => c.id === current)?.id || all[0]?.id || 0,
      );
    })()
      .catch((e) => {
        if (active) {
          setLoadFailed(true);
          setError(e.message || "期間を読み込めませんでした");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [revision]);

  React.useEffect(() => {
    let active = true;
    setGroups([]);
    setUsers({});
    setGroupsFailed(false);
    if (!cid) {
      setGroupsLoading(false);
      return;
    }
    setGroupsLoading(true);
    (async () => {
      const all: api.Group[] = [];
      for (let page = 1; ; page++) {
        // Nested user_ids also count toward the GraphQL complexity limit.
        const result = (await api.loadGroups(cid, page, 30))
          .adminDayshiftGroupsPaginated;
        all.push(...result.items);
        if (all.length >= result.total || !result.items.length) break;
      }
      const ids = [...new Set(all.flatMap((g) => g.user_ids))];
      const members = ids.length ? await api.loadUsersByIds(ids) : [];
      if (!active) return;
      setGroups(all.sort((a, b) => a.position - b.position || a.id - b.id));
      setUsers(Object.fromEntries(members.map((u) => [u.id, u])));
    })()
      .catch((e) => {
        if (active) {
          setGroupsFailed(true);
          setError(e.message || "グループを読み込めませんでした");
        }
      })
      .finally(() => {
        if (active) setGroupsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [cid, revision]);

  React.useEffect(() => {
    if (dialog !== "group") return;
    let active = true;
    setSearching(true);
    setSearchError("");
    setOptions([]);
    const timer = setTimeout(() => {
      api
        .searchAdminUsers(query.trim())
        .then((result) => {
          if (active) setOptions(result);
        })
        .catch(() => {
          if (active)
            setSearchError(
              "検索に失敗しました。検索語を入力し直してください。",
            );
        })
        .finally(() => {
          if (active) setSearching(false);
        });
    }, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [query, dialog]);

  async function run(action: () => Promise<unknown>, message: string) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
      setNotice(message);
      setDialog(null);
      setConfirmation(null);
      refresh();
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : "操作に失敗しました。もう一度お試しください。",
      );
    } finally {
      setBusy(false);
    }
  }
  function openGroup(group: api.Group | null) {
    setEditing(group);
    setQuery("");
    setSelected(
      group
        ? group.user_ids.map(
            (id) =>
              users[id] || { id, display_name: `ユーザー #${id}`, email: "" },
          )
        : [],
    );
    setDialog("group");
  }
  async function move(position: number, direction: number) {
    const ids = groups.map((g) => g.id);
    [ids[position], ids[position + direction]] = [
      ids[position + direction],
      ids[position],
    ];
    await api.reorderGroups(cid, ids);
  }
  const cycle = cycles.find((c) => c.id === cid);
  const ready =
    !loading && !groupsLoading && !busy && !loadFailed && !groupsFailed;
  const members = (g: api.Group) =>
    g.user_ids
      .map((id) => (users[id] ? label(users[id]) : `ユーザー #${id}`))
      .join("、") || "メンバーなし";
  const next = groups.length ? groups[index % groups.length] : null;
  const validIndex =
    /^\d+$/.test(indexDraft) && Number(indexDraft) <= 2147483647;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">日直管理</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            期間を選んで、担当メンバーと回る順番を管理します。
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => {
            setEditingCycle(null);
            setError("");
            setBegin("");
            setFinish("");
            setDescription("");
            setDialog("cycle");
          }}
          disabled={!ready}
        >
          <PlusIcon className="mr-2 size-4" />
          期間を作成
        </Button>
      </header>
      {error && (
        <div
          role="alert"
          className="rounded-lg border border-destructive p-4 text-sm text-destructive"
        >
          {error}
          <Button
            variant="outline"
            className="ml-3"
            disabled={busy}
            onClick={() => {
              setError("");
              refresh();
            }}
          >
            再読み込み
          </Button>
        </div>
      )}
      {notice && (
        <p role="status" className="rounded-lg bg-muted p-3 text-sm">
          {notice}
        </p>
      )}
      <Card className="gap-4 p-5">
        <div className="grid items-end gap-4 sm:grid-cols-[1fr_auto]">
          <div className="space-y-2">
            <label htmlFor="cycle" className="text-sm font-medium">
              管理する期間
            </label>
            <select
              id="cycle"
              className={selectClass}
              value={cid}
              disabled={!ready || !cycles.length}
              onChange={(e) => {
                setCid(Number(e.target.value));
                setNotice("");
              }}
            >
              {!cycles.length && (
                <option value={0}>
                  {loading ? "読み込み中…" : "期間がありません"}
                </option>
              )}
              {cycles.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.description || `期間 #${c.id}`}
                  {c.id === currentCid ? "（運用中）" : ""} / {c.begin_date} 〜{" "}
                  {c.finish_date}
                </option>
              ))}
            </select>
          </div>
          {cycle && (
            <span className="rounded-full bg-muted px-3 py-2 text-center text-sm">
              {cid === currentCid ? "運用中の期間" : "編集中の期間"}
            </span>
          )}
        </div>
        {cycle ? (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4">
            <p className="text-sm text-muted-foreground">
              {cycle.begin_date} 〜 {cycle.finish_date} ·{" "}
              {groupsLoading ? "読み込み中…" : `${groups.length} グループ`}
            </p>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                disabled={!ready}
                onClick={() => {
                  setEditingCycle(cycle);
                  setBegin(cycle.begin_date);
                  setFinish(cycle.finish_date);
                  setDescription(cycle.description);
                  setError("");
                  setDialog("cycle");
                }}
              >
                期間を編集
              </Button>
              {cid !== currentCid && (
                <Button
                  disabled={!ready || !groups.length}
                  onClick={() =>
                    setConfirmation({
                      title: "この期間に切り替えますか？",
                      description: `「${cycle.description}」を運用中の期間に設定します。現在の担当位置は引き継がれます。通知への適用は期間内のみです。`,
                      action: () => api.setCurrentCycle(cid),
                    })
                  }
                >
                  この期間を運用する
                </Button>
              )}
              <Button
                variant="ghost"
                disabled={!ready || cid === currentCid}
                onClick={() =>
                  setConfirmation({
                    title: "期間を削除しますか？",
                    description: `「${cycle.description}」と所属するグループを削除します。この操作は取り消せません。`,
                    action: () => api.deleteCycle(cid),
                  })
                }
              >
                期間を削除
              </Button>
            </div>
          </div>
        ) : (
          !loading && (
            <p className="text-sm text-muted-foreground">
              「期間を作成」から開始日・終了日を登録し、担当グループを追加しましょう。
            </p>
          )
        )}
      </Card>
      {cycle && (
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <section className="min-w-0 space-y-4" aria-labelledby="groups-title">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 id="groups-title" className="text-lg font-semibold">
                  担当グループ
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  上から順に担当します。矢印で順番を変更できます。
                </p>
              </div>
              <Button disabled={!ready} onClick={() => openGroup(null)}>
                <PlusIcon className="mr-2 size-4" />
                グループを追加
              </Button>
            </div>
            {groupsLoading ? (
              <p
                role="status"
                className="py-8 text-center text-muted-foreground"
              >
                グループを読み込み中…
              </p>
            ) : groups.length === 0 ? (
              <Card className="items-center p-8 text-center">
                <UsersIcon className="size-8 text-muted-foreground" />
                <h3 className="font-medium">最初のグループを追加しましょう</h3>
                <p className="text-sm text-muted-foreground">
                  一緒に担当するメンバーを選ぶだけで登録できます。
                </p>
              </Card>
            ) : (
              <ol className="space-y-3">
                {groups.map((g, position) => (
                  <li
                    key={g.id}
                    className="flex flex-wrap items-center gap-3 rounded-xl border bg-card p-4"
                  >
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-semibold">
                      {position + 1}
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="break-words text-sm font-medium">
                        {members(g)}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {g.user_ids.length} 人
                        {next?.id === g.id && cid === currentCid
                          ? " · 現在の担当"
                          : ""}
                      </p>
                    </div>
                    <div className="flex w-full justify-end gap-1 sm:w-auto">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`${position + 1}番のグループを上へ`}
                        disabled={!ready || position === 0}
                        onClick={() =>
                          run(() => move(position, -1), "順番を変更しました")
                        }
                      >
                        <ArrowUpIcon className="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`${position + 1}番のグループを下へ`}
                        disabled={!ready || position === groups.length - 1}
                        onClick={() =>
                          run(() => move(position, 1), "順番を変更しました")
                        }
                      >
                        <ArrowDownIcon className="size-4" />
                      </Button>
                      <Button
                        variant="outline"
                        disabled={!ready}
                        onClick={() => openGroup(g)}
                        aria-label={`${position + 1}番のグループを編集`}
                      >
                        編集
                      </Button>
                      <Button
                        variant="ghost"
                        disabled={!ready}
                        onClick={() =>
                          setConfirmation({
                            title: "グループを削除しますか？",
                            description: `${members(g)} のグループを削除します。削除すると担当順が変わることがあります。`,
                            action: () => api.deleteGroup(cid, g.id),
                          })
                        }
                        aria-label={`${position + 1}番のグループを削除`}
                      >
                        削除
                      </Button>
                    </div>
                  </li>
                ))}
              </ol>
            )}
            <Button
              variant="ghost"
              disabled={!ready}
              onClick={() => {
                setCsv("");
                setDialog("csv");
              }}
            >
              CSVでまとめて追加
            </Button>
          </section>
          <aside className="space-y-4">
            <Card className="gap-4 p-5">
              <h2 className="font-semibold">
                {cid === currentCid
                  ? "これからの担当順"
                  : "この期間の担当順プレビュー"}
              </h2>
              <p className="text-xs leading-relaxed text-muted-foreground">
                現在の担当位置から表示します。最後のグループの次は先頭に戻ります。休日や期間の切り替えによって、実際の担当日は変わります。設定した期間外では、日付に合う期間が自動で選ばれます。
              </p>
              {next && !groupsLoading ? (
                <ol className="space-y-4">
                  {Array.from(
                    { length: Math.min(groups.length, 5) },
                    (_, offset) => {
                      const g = groups[(index + offset) % groups.length];
                      return (
                        <li key={offset} className="flex gap-3">
                          <span className="w-12 shrink-0 text-xs text-muted-foreground">
                            {offset === 0 ? "現在" : `${offset}回後`}
                          </span>
                          <span className="break-words text-sm font-medium">
                            {members(g)}
                          </span>
                        </li>
                      );
                    },
                  )}
                </ol>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {groupsLoading
                    ? "読み込み中…"
                    : "グループを追加すると表示されます。"}
                </p>
              )}
            </Card>
            <details className="rounded-xl border p-5">
              <summary className="cursor-pointer text-sm font-medium">
                担当位置の調整
              </summary>
              <div className="mt-4 space-y-3">
                <p className="text-xs leading-relaxed text-muted-foreground">
                  運用中の期間に適用されます。通常は自動で進むため、担当を修正する場合だけ変更してください。0が最初のグループです。
                </p>
                <label htmlFor="shift-index" className="block text-sm">
                  担当位置（0から）
                </label>
                <Input
                  id="shift-index"
                  type="number"
                  min="0"
                  max="2147483647"
                  step="1"
                  value={indexDraft}
                  onChange={(e) => setIndexDraft(e.target.value)}
                />
                <Button
                  variant="outline"
                  disabled={
                    !ready ||
                    cid !== currentCid ||
                    !validIndex ||
                    Number(indexDraft) === index
                  }
                  onClick={() =>
                    setConfirmation({
                      title: "担当位置を変更しますか？",
                      description: `担当位置を ${index} から ${indexDraft} に変更します。通知される担当グループに影響します。`,
                      action: () => api.updateDayShiftIndex(Number(indexDraft)),
                    })
                  }
                >
                  担当位置を保存
                </Button>
              </div>
            </details>
          </aside>
        </div>
      )}
      <Dialog
        open={dialog !== null}
        onOpenChange={(open) => {
          if (!open && !busy) setDialog(null);
        }}
      >
        <DialogContent className="max-h-[85dvh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {dialog === "cycle"
                ? editingCycle
                  ? "期間を編集"
                  : "期間を作成"
                : dialog === "csv"
                  ? "CSVでグループを追加"
                  : editing
                    ? "メンバーを編集"
                    : "グループを追加"}
            </DialogTitle>
            <DialogDescription>
              {dialog === "cycle"
                ? editingCycle
                  ? "期間名・開始日・終了日を変更します。担当グループと順番はそのまま保持されます。"
                  : "日直を担当する期間と、見分けやすい名前を登録します。"
                : dialog === "csv"
                  ? "1行が1グループです。ユーザーIDをカンマ区切りで入力してください。"
                  : "一緒に担当するメンバーを選んでください。"}
            </DialogDescription>
          </DialogHeader>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          {dialog === "cycle" && (
            <div className="space-y-4">
              <label className="block space-y-2">
                <span>期間名</span>
                <Input
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="例：2026年度 2学期"
                  disabled={busy}
                />
              </label>
              <label className="block space-y-2">
                <span>開始日</span>
                <Input
                  type="date"
                  value={begin}
                  onChange={(e) => setBegin(e.target.value)}
                  disabled={busy}
                />
              </label>
              <label className="block space-y-2">
                <span>終了日</span>
                <Input
                  type="date"
                  value={finish}
                  min={begin}
                  onChange={(e) => setFinish(e.target.value)}
                  disabled={busy}
                />
              </label>
              {begin && finish && begin >= finish && (
                <p role="alert" className="text-sm text-destructive">
                  終了日は開始日より後の日付を選んでください。
                </p>
              )}
            </div>
          )}
          {dialog === "group" && (
            <div className="space-y-3">
              <p className="text-sm font-medium">
                選択中：{selected.length} 人
              </p>
              <div className="flex flex-wrap gap-2">
                {selected.map((u) => (
                  <Button
                    key={u.id}
                    size="sm"
                    variant="secondary"
                    disabled={busy}
                    aria-label={`${label(u)}の選択を解除`}
                    onClick={() =>
                      setSelected(selected.filter((v) => v.id !== u.id))
                    }
                  >
                    {label(u)} ×
                  </Button>
                ))}
              </div>
              <Input
                aria-label="メンバーを検索"
                placeholder="名前・メールアドレスで検索"
                value={query}
                disabled={busy}
                onChange={(e) => setQuery(e.target.value)}
              />
              <div className="max-h-56 space-y-1 overflow-y-auto rounded-md border p-2">
                {searching ? (
                  <p role="status" className="p-2 text-sm">
                    検索中…
                  </p>
                ) : searchError ? (
                  <p role="alert" className="p-2 text-sm text-destructive">
                    {searchError}
                  </p>
                ) : options.length === 0 ? (
                  <p className="p-2 text-sm text-muted-foreground">
                    一致するメンバーがいません
                  </p>
                ) : (
                  options.map((u) => (
                    <label
                      key={u.id}
                      className="flex cursor-pointer items-center gap-3 rounded p-2 hover:bg-muted"
                    >
                      <input
                        type="checkbox"
                        disabled={busy}
                        checked={selected.some((v) => v.id === u.id)}
                        onChange={(e) =>
                          setSelected(
                            e.target.checked
                              ? [...selected, u]
                              : selected.filter((v) => v.id !== u.id),
                          )
                        }
                      />
                      <span className="min-w-0 break-words text-sm">
                        {label(u)}
                        <span className="block text-xs text-muted-foreground">
                          {u.email}
                        </span>
                      </span>
                    </label>
                  ))
                )}
              </div>
            </div>
          )}
          {dialog === "csv" && (
            <Textarea
              aria-label="グループのCSV"
              placeholder={"12,34\n5,8,9"}
              rows={8}
              value={csv}
              disabled={busy}
              onChange={(e) => setCsv(e.target.value)}
            />
          )}
          <DialogFooter>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => setDialog(null)}
            >
              キャンセル
            </Button>
            <Button
              disabled={
                busy ||
                (dialog === "cycle"
                  ? !description.trim() || !begin || !finish || begin >= finish
                  : dialog === "group"
                    ? !selected.length
                    : !csv.trim())
              }
              onClick={() =>
                run(
                  async () => {
                    if (dialog === "cycle") {
                      if (editingCycle) {
                        await api.updateCycle(
                          editingCycle.id,
                          begin,
                          finish,
                          description.trim(),
                        );
                      } else {
                        const result = await api.createCycle(
                          begin,
                          finish,
                          description.trim(),
                        );
                        setCid(result.adminDayshiftCreateCycle.id);
                      }
                    }
                    if (dialog === "group") {
                      if (editing)
                        await api.updateGroupMembers(
                          editing.id,
                          selected.map((u) => u.id),
                        );
                      else
                        await api.createGroup(
                          cid,
                          selected.map((u) => u.id),
                        );
                    }
                    if (dialog === "csv") {
                      const lines = csv
                        .split(/\r?\n/)
                        .filter((line) => line.trim());
                      const rows = lines.map((line, i) => {
                        const tokens = line.split(",").map((s) => s.trim());
                        if (
                          tokens.some(
                            (s) =>
                              !/^\d+$/.test(s) ||
                              Number(s) < 1 ||
                              Number(s) > 2147483647,
                          )
                        )
                          throw new Error(
                            `${i + 1}行目のユーザーIDを確認してください。`,
                          );
                        return [...new Set(tokens.map(Number))];
                      });
                      for (let i = 0; i < rows.length; i++) {
                        try {
                          await api.createGroup(cid, rows[i]);
                        } catch (e) {
                          setCsv(lines.slice(i).join("\n"));
                          refresh();
                          throw new Error(
                            `${i}グループを追加しました。残りの行を修正して再実行してください。${e instanceof Error ? e.message : ""}`,
                          );
                        }
                      }
                    }
                  },
                  dialog === "cycle"
                    ? editingCycle
                      ? "期間を更新しました。"
                      : "期間を作成しました。グループを追加してください。"
                    : "グループを保存しました",
                )
              }
            >
              {busy ? "保存中…" : "保存"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!confirmation}
        onOpenChange={(open) => {
          if (!open && !busy) setConfirmation(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{confirmation?.title}</DialogTitle>
            <DialogDescription>{confirmation?.description}</DialogDescription>
          </DialogHeader>
          {error && (
            <p role="alert" className="text-destructive">
              {error}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => setConfirmation(null)}
            >
              キャンセル
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                confirmation && run(confirmation.action, "設定を更新しました")
              }
            >
              {busy ? "更新中…" : "実行する"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
