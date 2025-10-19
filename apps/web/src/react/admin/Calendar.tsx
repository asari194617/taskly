"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { MoreVertical } from "lucide-react";

export default function AdminCalendar() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [start, setStart] = React.useState("");
  const [end, setEnd] = React.useState("");
  const [isBiz, setIsBiz] = React.useState(true);
  const [notes, setNotes] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [msg, setMsg] = React.useState<string | null>(null);
  const [items, setItems] = React.useState<
    { date: string; is_business_day: boolean; notes?: string | null }[]
  >([]);
  const [page, setPage] = React.useState(1);
  const [pageSize] = React.useState(30);
  const [hasNext, setHasNext] = React.useState(false);
  const [hasPrev, setHasPrev] = React.useState(false);
  const [includePast, setIncludePast] = React.useState(false);
  const [order, setOrder] = React.useState<"asc" | "desc">("asc");
  const [q, setQ] = React.useState("");
  const [editOpen, setEditOpen] = React.useState(false);
  const [editDate, setEditDate] = React.useState<string | null>(null);
  const [editIsBiz, setEditIsBiz] = React.useState(true);
  const [editNotes, setEditNotes] = React.useState("");
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteDate, setDeleteDate] = React.useState<string | null>(null);

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
      await loadPaged(page);
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

  async function loadPaged(nextPage: number) {
    try {
      const d = await gql<{
        adminCalendarList: {
          items: {
            date: string;
            is_business_day: boolean;
            notes?: string | null;
          }[];
          page: number;
          page_size: number;
          total: number;
          has_next: boolean;
          has_prev: boolean;
        };
      }>(
        `query($p:Int,$ps:Int,$past:Boolean,$order:String,$q:String){
          adminCalendarList(page:$p,page_size:$ps,include_past:$past,order:$order,q:$q){
            items{ date is_business_day notes }
            page page_size total has_next has_prev
          }
        }`,
        {
          p: nextPage,
          ps: pageSize,
          past: includePast,
          order,
          q: q || undefined,
        }
      );
      const res = d.adminCalendarList;
      setItems(res.items || []);
      setPage(res.page || nextPage);
      setHasNext(!!res.has_next);
      setHasPrev(!!res.has_prev);
    } catch (err: any) {
      toast({
        variant: "destructive",
        title: "取得に失敗しました",
        description: err?.message,
      });
    }
  }

  function openEdit(it: {
    date: string;
    is_business_day: boolean;
    notes?: string | null;
  }) {
    setEditDate(it.date);
    setEditIsBiz(!!it.is_business_day);
    setEditNotes(it.notes || "");
    setEditOpen(true);
  }

  async function saveEdit() {
    if (!editDate) return;
    try {
      setBusy(true);
      await gql<{ adminCalendarUpsertRange: boolean }>(
        `mutation($s:String!,$e:String!,$b:Boolean!,$n:String){ adminCalendarUpsertRange(start:$s,end:$e,is_business_day:$b,notes:$n) }`,
        { s: editDate, e: editDate, b: editIsBiz, n: editNotes || undefined }
      );
      setEditOpen(false);
      await loadPaged(page);
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

  function openDelete(date: string) {
    setDeleteDate(date);
    setDeleteOpen(true);
  }

  async function deleteEntry() {
    if (!deleteDate) return;
    try {
      setBusy(true);
      await gql<{ adminCalendarDelete: boolean }>(
        `mutation($d:String!){ adminCalendarDelete(date:$d) }`,
        { d: deleteDate }
      );
      setDeleteOpen(false);
      await loadPaged(page);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    } finally {
      setBusy(false);
    }
  }

  React.useEffect(() => {
    if (token) {
      loadPaged(1);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, includePast, order, q]);

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">営業日カレンダー</h1>

      {/* Input card: range + apply */}
      <Card className="p-4 space-y-4">
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

      {/* Filter/Search + List card */}
      <Card className="p-4 space-y-4">
        {/* Toolbar */}
        <div className="flex flex-col sm:flex-row gap-2 sm:items-end sm:justify-between">
          <div className="flex-1">
            <InputGroup>
              <InputGroupInput
                placeholder="日付(YYYY-MM-DD) または メモで検索"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") loadPaged(1);
                }}
              />
              <InputGroupAddon align="inline-end">
                {items.length} 件
              </InputGroupAddon>
            </InputGroup>
          </div>
          <div className="flex gap-3 items-center">
            <div className="flex items-center gap-2">
              <Switch
                checked={includePast}
                onCheckedChange={(v: boolean) => setIncludePast(v)}
              />
              <span className="text-sm">過去も含める</span>
            </div>
            <div>
              <Select value={order} onValueChange={(v: any) => setOrder(v)}>
                <SelectTrigger className="w-[140px]">
                  <SelectValue placeholder="並び順" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="asc">日付 昇順</SelectItem>
                  <SelectItem value="desc">日付 降順</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        {/* Paged entries */}
        <div className="pt-2">
          {items.length === 0 ? (
            <div className="text-sm text-muted-foreground">
              該当する日付がありません
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left">
                    <th className="py-2 px-1">日付</th>
                    <th className="py-2 px-1">営業日</th>
                    <th className="py-2 px-1">メモ</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((it) => (
                    <tr key={it.date} className="border-t">
                      <td className="py-1 px-1 whitespace-nowrap">{it.date}</td>
                      <td className="py-1 px-1">
                        {it.is_business_day ? "Yes" : "No"}
                      </td>
                      <td className="py-1 px-1 flex items-center justify-between gap-2">
                        <span className="truncate">{it.notes || ""}</span>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              size="icon"
                              variant="ghost"
                              aria-label="操作"
                            >
                              <MoreVertical className="h-4 w-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={() => openEdit(it)}>
                              編集
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              className="text-destructive focus:text-destructive"
                              onClick={() => openDelete(it.date)}
                            >
                              削除
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <Pagination className="mt-3">
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  href="#"
                  onClick={(e) => {
                    e.preventDefault();
                    if (hasPrev) loadPaged(page - 1);
                  }}
                  className={!hasPrev ? "pointer-events-none opacity-50" : ""}
                />
              </PaginationItem>
              <PaginationItem>
                <PaginationLink href="#" isActive>
                  {page}
                </PaginationLink>
              </PaginationItem>
              <PaginationItem>
                <PaginationNext
                  href="#"
                  onClick={(e) => {
                    e.preventDefault();
                    if (hasNext) loadPaged(page + 1);
                  }}
                  className={!hasNext ? "pointer-events-none opacity-50" : ""}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      </Card>

      {/* Edit dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>予定を編集</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div>
              <div className="text-sm mb-1">日付</div>
              <Input
                type="date"
                value={editDate || ""}
                onChange={(e) => setEditDate(e.target.value)}
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                checked={editIsBiz}
                onCheckedChange={(v: boolean) => setEditIsBiz(v)}
              />
              <span className="text-sm">営業日</span>
            </div>
            <div>
              <div className="text-sm mb-1">メモ</div>
              <Input
                value={editNotes}
                onChange={(e) => setEditNotes(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              キャンセル
            </Button>
            <Button onClick={saveEdit} disabled={busy || !editDate}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>削除の確認</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">
              {deleteDate} を削除します。よろしいですか？
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
              キャンセル
            </Button>
            <Button variant="destructive" onClick={deleteEntry} disabled={busy}>
              削除する
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
