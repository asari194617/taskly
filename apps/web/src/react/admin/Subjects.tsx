"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogClose,
  DialogDescription,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { DateTimePicker } from "@/components/ui/date-time-picker";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerDescription,
  DrawerFooter,
  DrawerTrigger,
  DrawerClose,
} from "@/components/ui/drawer";

type Subject = {
  id: number;
  name_ja: string;
  name_en?: string | null;
  created_at: string;
  expires_at?: string | null;
};

function toRFC3339(input: string): string | undefined {
  const v = (input || "").trim();
  if (!v) return undefined;
  const d = new Date(v);
  if (isNaN(d.getTime())) return undefined;
  return d.toISOString();
}
function fromRFC3339(s?: string | null): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(
    d.getHours()
  )}:${pad(d.getMinutes())}`;
}

export default function AdminSubjects() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [items, setItems] = React.useState<Subject[]>([]);
  const [loading, setLoading] = React.useState(false);

  async function load() {
    try {
      setLoading(true);
      const data = await gql<{ adminSubjects: Subject[] }>(
        `query($inc:Boolean){ adminSubjects(include_expired:$inc){ id name_ja name_en created_at expires_at } }`,
        { inc: true }
      );
      setItems(data.adminSubjects || []);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoading(false);
    }
  }
  React.useEffect(() => {
    if (token) load();
  }, [token]);

  const isMobile = useIsMobile();

  // Create dialog states
  const [openCreate, setOpenCreate] = React.useState(false);
  const [nameJa, setNameJa] = React.useState("");
  const [nameEn, setNameEn] = React.useState("");
  const [expires, setExpires] = React.useState<string>("");

  async function create() {
    try {
      if (!nameJa.trim()) {
        toast({
          variant: "destructive",
          title: "名前(日本語)を入力してください",
        });
        return;
      }
      const data = await gql<{ adminSubjectCreate: Subject }>(
        `mutation($ja:String!,$en:String,$e:String){ adminSubjectCreate(name_ja:$ja, name_en:$en, expires_at:$e){ id name_ja name_en created_at expires_at } }`,
        {
          ja: nameJa.trim(),
          en: nameEn.trim() || undefined,
          e: toRFC3339(expires),
        }
      );
      setOpenCreate(false);
      setNameJa("");
      setNameEn("");
      setExpires("");
      setItems((prev) => [data.adminSubjectCreate, ...prev]);
      toast({ title: "作成しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }

  // Edit dialog states
  const [editOpen, setEditOpen] = React.useState(false);
  const [editItem, setEditItem] = React.useState<Subject | null>(null);
  const [editNameJa, setEditNameJa] = React.useState("");
  const [editNameEn, setEditNameEn] = React.useState("");
  const [editExpires, setEditExpires] = React.useState<string>("");

  function openEdit(it: Subject) {
    setEditItem(it);
    setEditNameJa(it.name_ja || "");
    setEditNameEn(it.name_en || "");
    setEditExpires(fromRFC3339(it.expires_at));
    setEditOpen(true);
  }

  async function saveEdit() {
    if (!editItem) return;
    try {
      const data = await gql<{ adminSubjectUpdate: Subject }>(
        `mutation($id:Int!,$ja:String,$en:String,$e:String){ adminSubjectUpdate(id:$id, name_ja:$ja, name_en:$en, expires_at:$e){ id name_ja name_en created_at expires_at } }`,
        {
          id: editItem.id,
          ja: editNameJa.trim() || undefined,
          en: editNameEn.trim() || undefined,
          e: toRFC3339(editExpires),
        }
      );
      setItems((prev) =>
        prev.map((p) => (p.id === editItem.id ? data.adminSubjectUpdate : p))
      );
      setEditOpen(false);
      setEditItem(null);
      toast({ title: "更新しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }

  async function softDelete(id: number) {
    try {
      await gql<{ adminSubjectDelete: boolean }>(
        `mutation($id:Int!){ adminSubjectDelete(id:$id) }`,
        { id }
      );
      // reflect soft delete by updating expires_at to now in UI
      setItems((prev) =>
        prev.map((p) =>
          p.id === id ? { ...p, expires_at: new Date().toISOString() } : p
        )
      );
      toast({ title: "削除(アーカイブ)しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">教科管理</h1>
        {isMobile ? (
          <Drawer open={openCreate} onOpenChange={setOpenCreate}>
            <DrawerTrigger asChild>
              <Button>教科を追加</Button>
            </DrawerTrigger>
            <DrawerContent>
              <DrawerHeader className="text-left">
                <DrawerTitle>教科を追加</DrawerTitle>
              </DrawerHeader>
              <div className="px-4 pb-4 grid gap-3">
                <div>
                  <div className="text-sm mb-1">名前(日本語)</div>
                  <Input
                    value={nameJa}
                    onChange={(e) => setNameJa(e.target.value)}
                    placeholder="例: 数学"
                  />
                </div>
                <div>
                  <div className="text-sm mb-1">名前(英語・任意)</div>
                  <Input
                    value={nameEn}
                    onChange={(e) => setNameEn(e.target.value)}
                    placeholder="例: Mathematics"
                  />
                </div>
                <div>
                  <div className="text-sm mb-1">有効期限(任意)</div>
                  <DateTimePicker
                    value={expires}
                    onChange={setExpires}
                    placeholder="日時を選択"
                  />
                </div>
              </div>
              <DrawerFooter>
                <DrawerClose asChild>
                  <Button variant="secondary">キャンセル</Button>
                </DrawerClose>
                <Button onClick={create}>作成</Button>
              </DrawerFooter>
            </DrawerContent>
          </Drawer>
        ) : (
          <Dialog open={openCreate} onOpenChange={setOpenCreate}>
            <DialogTrigger asChild>
              <Button>教科を追加</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>教科を追加</DialogTitle>
              </DialogHeader>
              <div className="grid gap-3">
                <div>
                  <div className="text-sm mb-1">名前(日本語)</div>
                  <Input
                    value={nameJa}
                    onChange={(e) => setNameJa(e.target.value)}
                    placeholder="例: 数学"
                  />
                </div>
                <div>
                  <div className="text-sm mb-1">名前(英語・任意)</div>
                  <Input
                    value={nameEn}
                    onChange={(e) => setNameEn(e.target.value)}
                    placeholder="例: Mathematics"
                  />
                </div>
                <div>
                  <div className="text-sm mb-1">有効期限(任意)</div>
                  <DateTimePicker
                    value={expires}
                    onChange={setExpires}
                    placeholder="日時を選択"
                  />
                </div>
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="secondary">キャンセル</Button>
                </DialogClose>
                <Button onClick={create}>作成</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        )}
      </div>

      <Card className="p-0 overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>名前(日本語)</TableHead>
              <TableHead>名前(英語)</TableHead>
              <TableHead>作成日</TableHead>
              <TableHead>期限</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((it) => (
              <TableRow
                key={it.id}
                className={it.expires_at ? "opacity-60" : ""}
              >
                <TableCell>{it.id}</TableCell>
                <TableCell className="max-w-[280px] truncate">
                  {it.name_ja}
                </TableCell>
                <TableCell className="max-w-[280px] truncate">
                  {it.name_en || "-"}
                </TableCell>
                <TableCell>
                  {new Date(it.created_at).toLocaleString()}
                </TableCell>
                <TableCell>
                  {it.expires_at
                    ? new Date(it.expires_at).toLocaleString()
                    : "-"}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex gap-2 justify-end">
                    <Button variant="outline" onClick={() => openEdit(it)}>
                      編集
                    </Button>
                    <Button
                      variant="destructive"
                      onClick={() => softDelete(it.id)}
                    >
                      削除
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>

      {isMobile ? (
        <Drawer open={editOpen} onOpenChange={setEditOpen}>
          <DrawerContent>
            <DrawerHeader className="text-left">
              <DrawerTitle>教科を編集</DrawerTitle>
              <DrawerDescription>
                空欄で保存すると期限を解除します。
              </DrawerDescription>
            </DrawerHeader>
            <div className="px-4 pb-4 grid gap-3">
              <div>
                <div className="text-sm mb-1">名前(日本語)</div>
                <Input
                  value={editNameJa}
                  onChange={(e) => setEditNameJa(e.target.value)}
                />
              </div>
              <div>
                <div className="text-sm mb-1">名前(英語・任意)</div>
                <Input
                  value={editNameEn}
                  onChange={(e) => setEditNameEn(e.target.value)}
                />
              </div>
              <div>
                <div className="text-sm mb-1">有効期限(任意)</div>
                <DateTimePicker
                  value={editExpires}
                  onChange={setEditExpires}
                  placeholder="日時を選択"
                />
              </div>
            </div>
            <DrawerFooter>
              <DrawerClose asChild>
                <Button variant="secondary">キャンセル</Button>
              </DrawerClose>
              <Button onClick={saveEdit}>保存</Button>
            </DrawerFooter>
          </DrawerContent>
        </Drawer>
      ) : (
        <Dialog open={editOpen} onOpenChange={setEditOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>教科を編集</DialogTitle>
              <DialogDescription>
                空欄で保存すると期限を解除します。
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-3">
              <div>
                <div className="text-sm mb-1">名前(日本語)</div>
                <Input
                  value={editNameJa}
                  onChange={(e) => setEditNameJa(e.target.value)}
                />
              </div>
              <div>
                <div className="text-sm mb-1">名前(英語・任意)</div>
                <Input
                  value={editNameEn}
                  onChange={(e) => setEditNameEn(e.target.value)}
                />
              </div>
              <div>
                <div className="text-sm mb-1">有効期限(任意)</div>
                <DateTimePicker
                  value={editExpires}
                  onChange={setEditExpires}
                  placeholder="日時を選択"
                />
              </div>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="secondary">キャンセル</Button>
              </DialogClose>
              <Button onClick={saveEdit}>保存</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
