import * as React from "react";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "../components/ui/empty";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
// Replaced DateTimePicker with period-based DeadlinePicker
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer";
import {
  ButtonGroup,
  ButtonGroupSeparator,
} from "@/components/ui/button-group";
import { useMediaQuery } from "@/react/hooks/useMediaQuery";
import { Kbd } from "../components/ui/kbd";
import {
  DeadlinePicker,
  type DeadlineMeta,
  sinceUntilFromMeta,
  deriveLabelFromEnums,
} from "@/components/DeadlinePicker";
import { Plus, CalendarDays } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertCircle,
  Hourglass,
  FileText,
  Pencil,
  Trash2,
  Save,
  X,
  MoreVertical,
} from "lucide-react";

type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline?: string | null;
  deadline_since?: string | null;
  deadline_until?: string | null;
  description?: string | null;
  type?: string | null;
  is_require_hand_in?: boolean;
  self_check_required?: boolean;
};
type Subject = { id: number; title: string };

// Slot-based helpers removed; we now use symbolic meta via DeadlinePicker

// RFC3339 helpers removed — we now carry date string directly

// slotFromRFC3339 removed

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function formatHeadingForDate(d: Date): string {
  const today = startOfDay(new Date());
  const diffMs = startOfDay(d).getTime() - today.getTime();
  const diffDays = Math.round(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays === 0) return "今日";
  if (diffDays === 1) return "あす";
  if (diffDays === 2) return "あさって";
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}/${m}/${day}`;
}

function formatDateYMDSlash(s: string): string {
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/);
  return m ? `${m[1]}/${m[2]}/${m[3]}` : s;
}

export default function TasksList() {
  const [items, setItems] = React.useState<Task[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [q, setQ] = React.useState("");
  const [creating, setCreating] = React.useState(false);
  const [newTitle, setNewTitle] = React.useState("");
  const [newSubjectId, setNewSubjectId] = React.useState("");
  // deadline as date + symbolic meta
  const [newDate, setNewDate] = React.useState(""); // yyyy-MM-dd
  const [newMeta, setNewMeta] = React.useState<DeadlineMeta | null>(null);
  const [newRequire, setNewRequire] = React.useState(false);
  const [newSelfCheck, setNewSelfCheck] = React.useState(false);
  const [newType, setNewType] = React.useState("");
  const [newDesc, setNewDesc] = React.useState("");
  const [createOpen, setCreateOpen] = React.useState(false);
  const [editingId, setEditingId] = React.useState<number | null>(null);
  const [editTitle, setEditTitle] = React.useState("");
  const [editSubjectId, setEditSubjectId] = React.useState("");
  const [editDate, setEditDate] = React.useState("");
  const [editMeta, setEditMeta] = React.useState<DeadlineMeta | null>(null);
  const [editRequire, setEditRequire] = React.useState(false);
  const [editSelfCheck, setEditSelfCheck] = React.useState(false);
  const [editType, setEditType] = React.useState("");
  const [editDesc, setEditDesc] = React.useState("");
  const [subjects, setSubjects] = React.useState<Subject[]>([]);
  const isDesktop = useMediaQuery("(min-width: 768px)");
  React.useEffect(() => {
    const token = localStorage.getItem("api_token");
    if (!token) {
      window.location.replace("/login");
      return;
    }
    Promise.all([
      gql<{ tasks: { items: Task[] } }>(
        `query($q:String){ tasks(limit:20, q:$q){ items{ id title subject_id deadline deadline_since deadline_until description type is_require_hand_in self_check_required } } }`,
        { q: q || undefined }
      ),
      gql<{ subjects: Subject[] }>(`query{ subjects{ id title } }`),
    ])
      .then(([t, s]) => {
        setItems(t.tasks.items);
        setSubjects(s.subjects || []);
      })
      .catch((e) => setError(e?.message || "読み込みに失敗しました"));
  }, [q]);

  // debounce search input changes
  const [qInput, setQInput] = React.useState("");
  React.useEffect(() => {
    const id = setTimeout(() => setQ(qInput.trim()), 300);
    return () => clearTimeout(id);
  }, [qInput]);

  // Keyboard shortcuts: g then n to create; '?' to show help
  React.useEffect(() => {
    let lastG = 0;
    function onKey(e: KeyboardEvent) {
      const target = e.target as HTMLElement | null;
      if (
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.getAttribute("contenteditable") === "true")
      )
        return;
      if (e.key.toLowerCase() === "g") {
        lastG = Date.now();
      } else if (e.key.toLowerCase() === "n") {
        if (Date.now() - lastG < 800) {
          setCreateOpen(true);
          lastG = 0;
        }
      } else if (e.key === "?") {
        toast({
          title: "ショートカット",
          description: "g → n: 新規作成 / ?: このヘルプ",
        });
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  function SubjectSelect(props: {
    value: string;
    onChange: (v: string) => void;
    subjects: Subject[];
    placeholder?: string;
  }) {
    const { value, onChange, subjects, placeholder } = props;
    const [open, setOpen] = React.useState(false);
    const [filter, setFilter] = React.useState("");
    const selected = subjects.find((s) => String(s.id) === value);
    const filtered = subjects.filter((s) =>
      s.title.toLowerCase().includes(filter.toLowerCase())
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
              {selected ? selected.title : placeholder || "選択してください"}
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
              placeholder="検索"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
            />
          </div>
          <div className="max-h-60 overflow-auto">
            {filtered.map((s) => (
              <DropdownMenuItem
                key={s.id}
                onClick={() => {
                  onChange(String(s.id));
                  setOpen(false);
                }}
              >
                {s.title}
              </DropdownMenuItem>
            ))}
            {filtered.length === 0 && (
              <div className="px-2 py-6 text-center text-sm text-muted-foreground">
                見つかりませんでした
              </div>
            )}
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }

  // removed old datetime helpers

  async function createTask() {
    if (!newTitle.trim() || !newSubjectId.trim() || !newDate.trim()) {
      toast({
        variant: "destructive",
        title: "タイトル・サブジェクト・期限（日付）は必須です",
      });
      return;
    }
    setCreating(true);
    try {
      const res = await gql<{ createTask: Task }>(
        `mutation($title:String!,$subject_id:Int!,$is:Boolean,$scr:Boolean,$deadline:String!,$deadline_since:String,$deadline_until:String,$description:String,$type:String){
          createTask(title:$title, subject_id:$subject_id, is_require_hand_in:$is, self_check_required:$scr, deadline:$deadline, deadline_since:$deadline_since, deadline_until:$deadline_until, description:$description, type:$type){ id title subject_id deadline deadline_since deadline_until description type is_require_hand_in self_check_required }
        }`,
        {
          title: newTitle.trim(),
          subject_id: parseInt(newSubjectId, 10),
          is: newRequire,
          scr: newSelfCheck,
          deadline: newDate,
          ...sinceUntilFromMeta(newDate, newMeta), // enums
          description: newDesc || undefined,
          type: newType || undefined,
        }
      );
      setItems((prev) => (prev ? [res.createTask, ...prev] : [res.createTask]));
      setNewTitle("");
      setNewSubjectId("");
      setNewDate("");
      setNewMeta(null);
      setNewRequire(false);
      setNewSelfCheck(false);
      setNewType("");
      setNewDesc("");
      setCreateOpen(false);
      toast({ title: "作成しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    } finally {
      setCreating(false);
    }
  }

  function beginEdit(t: Task) {
    setEditingId(t.id);
    setEditTitle(t.title);
    setEditSubjectId(String(t.subject_id || ""));
    setEditDate(t.deadline || "");
    setEditMeta(null);
    setEditRequire(!!t.is_require_hand_in);
    setEditSelfCheck(!!t.self_check_required);
    setEditType(t.type || "");
    setEditDesc(t.description || "");
  }
  function cancelEdit() {
    setEditingId(null);
    setEditTitle("");
    setEditSubjectId("");
    setEditDate("");
    setEditMeta(null);
    setEditRequire(false);
  }
  async function saveEdit(id: number) {
    const prev = items?.slice() || [];
    // optimistic update
    setItems((cur) =>
      (cur || []).map((x) =>
        x.id === id
          ? {
              ...x,
              title: editTitle,
              subject_id: parseInt(editSubjectId || "0", 10) || x.subject_id,
              deadline: editDate || null,
              // keep since/until in local state if needed (not shown here)
              is_require_hand_in: editRequire,
              self_check_required: editSelfCheck,
              type: editType,
              description: editDesc,
            }
          : x
      )
    );
    try {
      await gql<{ updateTask: Task }>(
        `mutation($id:Int!,$title:String,$subject_id:Int,$is:Boolean,$scr:Boolean,$deadline:String,$deadline_since:String,$deadline_until:String,$description:String,$type:String){
          updateTask(id:$id, title:$title, subject_id:$subject_id, is_require_hand_in:$is, self_check_required:$scr, deadline:$deadline, deadline_since:$deadline_since, deadline_until:$deadline_until, description:$description, type:$type){ id }
        }`,
        {
          id,
          title: editTitle,
          subject_id: editSubjectId ? parseInt(editSubjectId, 10) : undefined,
          is: editRequire,
          scr: editSelfCheck,
          deadline: editDate || undefined,
          ...sinceUntilFromMeta(editDate, editMeta),
          description: editDesc,
          type: editType,
        }
      );
      toast({ title: "更新しました" });
      cancelEdit();
    } catch (e: any) {
      // revert
      setItems(prev);
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }
  async function deleteTask(id: number) {
    const prev = items?.slice() || [];
    // optimistic remove
    setItems((cur) => (cur || []).filter((x) => x.id !== id));
    try {
      await gql<{ deleteTask: boolean }>(
        `mutation($id:Int!){ deleteTask(id:$id) }`,
        { id }
      );
      toast({ title: "削除しました" });
    } catch (e: any) {
      setItems(prev);
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }

  if (error) {
    return (
      <Empty className="border">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <AlertCircle className="h-5 w-5" />
          </EmptyMedia>
          <EmptyTitle>読み込みに失敗しました</EmptyTitle>
          <EmptyDescription>{error}</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button onClick={() => window.location.reload()}>再読み込み</Button>
        </EmptyContent>
      </Empty>
    );
  }

  if (!items) {
    return (
      <Empty className="border">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Hourglass className="h-5 w-5" />
          </EmptyMedia>
          <EmptyTitle>読み込み中</EmptyTitle>
          <EmptyDescription>課題を取得しています…</EmptyDescription>
        </EmptyHeader>
      </Empty>
    );
  }

  if (items.length === 0) {
    return (
      <Empty className="border">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <FileText className="h-5 w-5" />
          </EmptyMedia>
          <EmptyTitle>課題はありません</EmptyTitle>
          <EmptyDescription>
            新しい課題を追加するとここに表示されます。
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <div className="w-full max-w-xl text-left space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="col-span-2">
                <Label htmlFor="nt-title">タイトル</Label>
                <Input
                  id="nt-title"
                  value={newTitle}
                  onChange={(e) => setNewTitle(e.target.value)}
                  placeholder="課題のタイトル"
                />
              </div>
              <div>
                <Label htmlFor="nt-subject">サブジェクト</Label>
                <SubjectSelect
                  value={newSubjectId}
                  onChange={setNewSubjectId}
                  subjects={subjects}
                  placeholder="選択してください"
                />
              </div>
              <div>
                <Label htmlFor="nt-date">期限</Label>
                <DeadlinePicker
                  date={newDate}
                  meta={newMeta}
                  onChange={(d, m) => {
                    setNewDate(d);
                    setNewMeta(m);
                  }}
                />
              </div>
              <div className="col-span-2 flex items-center gap-2">
                <Switch
                  id="nt-req"
                  checked={newRequire}
                  onCheckedChange={setNewRequire}
                />
                <Label htmlFor="nt-req">提出必須</Label>
              </div>
              <div className="col-span-2 flex items-center gap-2 -mt-2">
                <Switch
                  id="nt-scr"
                  checked={newSelfCheck}
                  onCheckedChange={setNewSelfCheck}
                />
                <Label htmlFor="nt-scr">自己採点が必要</Label>
              </div>
              <div>
                <Label htmlFor="nt-type">タイプ</Label>
                <Select
                  value={newType}
                  onValueChange={(v: string) => setNewType(v)}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="選択してください" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="オンライン">オンライン</SelectItem>
                    <SelectItem value="対面">対面</SelectItem>
                    <SelectItem value="小テスト">小テスト</SelectItem>
                    <SelectItem value="発表">発表</SelectItem>
                    <SelectItem value="自習/予習">自習/予習</SelectItem>
                    <SelectItem value="その他">その他</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="col-span-2 mt-0">
                <Label htmlFor="nt-desc">説明（任意）</Label>
                <textarea
                  id="nt-desc"
                  className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                  rows={3}
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                />
              </div>
              {/* 自己採点は提出必須の直下に移動済み */}
            </div>
            <Button onClick={createTask} disabled={creating}>
              新規作成
            </Button>
          </div>
        </EmptyContent>
      </Empty>
    );
  }

  return (
    <div className="space-y-6">
      {/* Toolbar */}
      {items.length > 0 && (
        <div className="flex items-center justify-end gap-2">
          <div className="flex-1">
            <Input
              placeholder="課題を検索 (タイトル/説明)"
              value={qInput}
              onChange={(e) => setQInput(e.target.value)}
            />
          </div>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  size="sm"
                  onClick={() => setCreateOpen(true)}
                  className="sm:inline-flex hidden"
                >
                  <Plus className="h-4 w-4 mr-1" /> 新規作成
                </Button>
              </TooltipTrigger>
              <TooltipContent>新しい課題を作成</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          {/* Mobile icon button */}
          <Button
            size="icon"
            className="sm:hidden inline-flex"
            onClick={() => setCreateOpen(true)}
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>
      )}

      {/* New Task Form (responsive dialog/drawer) */}
      <Dialog
        open={isDesktop && createOpen}
        onOpenChange={(v: boolean) => setCreateOpen(isDesktop ? v : createOpen)}
      >
        <DialogContent className="sm:max-w-[640px]">
          <DialogHeader>
            <DialogTitle>新規課題の作成</DialogTitle>
            <DialogDescription>
              必要な情報を入力してください。
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3">
            <div className="col-span-2">
              <Label htmlFor="nt-title-2">タイトル</Label>
              <Input
                id="nt-title-2"
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                placeholder="課題のタイトル"
              />
            </div>
            <div>
              <Label htmlFor="nt-subject-2">サブジェクト</Label>
              <SubjectSelect
                value={newSubjectId}
                onChange={setNewSubjectId}
                subjects={subjects}
                placeholder="選択してください"
              />
            </div>
            <div>
              <Label htmlFor="nt-date-2">期限</Label>
              <DeadlinePicker
                date={newDate}
                meta={newMeta}
                onChange={(d, m) => {
                  setNewDate(d);
                  setNewMeta(m);
                }}
              />
            </div>
            <div className="col-span-2 flex items-center gap-2">
              <Switch
                id="nt-req-2"
                checked={newRequire}
                onCheckedChange={setNewRequire}
              />
              <Label htmlFor="nt-req-2">提出必須</Label>
            </div>
            <div className="col-span-2 flex items-center gap-2 -mt-2">
              <Switch
                id="nt-scr-2"
                checked={newSelfCheck}
                onCheckedChange={setNewSelfCheck}
              />
              <Label htmlFor="nt-scr-2">自己採点が必要</Label>
            </div>
            <div>
              <Label htmlFor="nt-type-2">タイプ</Label>
              <Select
                value={newType}
                onValueChange={(v: string) => setNewType(v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="選択してください" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="オンライン">オンライン</SelectItem>
                  <SelectItem value="対面">対面</SelectItem>
                  <SelectItem value="小テスト">小テスト</SelectItem>
                  <SelectItem value="発表">発表</SelectItem>
                  <SelectItem value="自習/予習">自習/予習</SelectItem>
                  <SelectItem value="その他">その他</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="col-span-2 mt-0">
              <Label htmlFor="nt-desc-2">説明（任意）</Label>
              <textarea
                id="nt-desc-2"
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                rows={3}
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
              />
            </div>
            {/* 自己採点は提出必須の直下に移動済み */}
          </div>
          <div className="mt-3">
            <Button onClick={createTask} disabled={creating}>
              新規作成
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Drawer
        open={!isDesktop && createOpen}
        onOpenChange={(v: boolean) =>
          setCreateOpen(!isDesktop ? v : createOpen)
        }
      >
        <DrawerContent>
          <DrawerHeader className="text-left">
            <DrawerTitle>新規課題の作成</DrawerTitle>
            <DrawerDescription>
              必要な情報を入力してください。
            </DrawerDescription>
          </DrawerHeader>
          <div className="px-4 pb-4 grid grid-cols-2 gap-3">
            {/* form fields are the same as dialog */}
            <div className="col-span-2">
              <Label>タイトル</Label>
              <Input
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                placeholder="課題のタイトル"
              />
            </div>
            <div>
              <Label>サブジェクト</Label>
              <SubjectSelect
                value={newSubjectId}
                onChange={setNewSubjectId}
                subjects={subjects}
                placeholder="選択してください"
              />
            </div>
            <div>
              <Label>期限</Label>
              <DeadlinePicker
                date={newDate}
                meta={newMeta}
                onChange={(d, m) => {
                  setNewDate(d);
                  setNewMeta(m);
                }}
              />
            </div>
            <div className="col-span-2 flex items-center gap-2">
              <Switch
                id="nt-req-m"
                checked={newRequire}
                onCheckedChange={setNewRequire}
              />
              <Label htmlFor="nt-req-m">提出必須</Label>
            </div>
            <div className="col-span-2 flex items-center gap-2 -mt-2">
              <Switch
                id="nt-scr-m"
                checked={newSelfCheck}
                onCheckedChange={setNewSelfCheck}
              />
              <Label htmlFor="nt-scr-m">自己採点が必要</Label>
            </div>
            <div>
              <Label>タイプ</Label>
              <Select
                value={newType}
                onValueChange={(v: string) => setNewType(v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="選択してください" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="オンライン">オンライン</SelectItem>
                  <SelectItem value="対面">対面</SelectItem>
                  <SelectItem value="小テスト">小テスト</SelectItem>
                  <SelectItem value="発表">発表</SelectItem>
                  <SelectItem value="自習/予習">自習/予習</SelectItem>
                  <SelectItem value="その他">その他</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="col-span-2">
              <Label>説明（任意）</Label>
              <textarea
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                rows={3}
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
              />
            </div>
            {/* 自己採点は提出必須の直下に配置済み */}
            <div className="col-span-2">
              <Button
                onClick={createTask}
                disabled={creating}
                className="w-full"
              >
                作成
              </Button>
            </div>
          </div>
        </DrawerContent>
      </Drawer>
      {/* Grouped List by date headings */}
      {(() => {
        const groups: {
          key: string;
          sortKey: number;
          items: Task[];
          heading: string;
        }[] = [];
        const byKey = new Map<
          string,
          { key: string; sortKey: number; items: Task[]; heading: string }
        >();
        const todayStart = startOfDay(new Date()).getTime();
        for (const t of items) {
          if (!t.deadline) {
            const key = "期限なし";
            if (!byKey.has(key)) {
              byKey.set(key, {
                key,
                sortKey: Number.MAX_SAFE_INTEGER,
                items: [],
                heading: key,
              });
            }
            byKey.get(key)!.items.push(t);
            continue;
          }
          const d = new Date(t.deadline);
          const sod = startOfDay(d).getTime();
          const heading = formatHeadingForDate(d);
          const key = String(sod);
          if (!byKey.has(key)) {
            byKey.set(key, { key, sortKey: sod, items: [], heading });
          }
          byKey.get(key)!.items.push(t);
        }
        for (const g of byKey.values()) groups.push(g);
        groups.sort((a, b) => a.sortKey - b.sortKey);
        return (
          <div className="space-y-6">
            {groups.map((g) => (
              <div key={g.key}>
                <h3 className="text-sm font-medium text-muted-foreground mb-2">
                  {g.heading}
                </h3>
                <ul className="space-y-3">
                  {g.items
                    .slice()
                    .sort((a, b) => {
                      const ta = a.deadline
                        ? new Date(a.deadline).getTime()
                        : Number.MAX_SAFE_INTEGER;
                      const tb = b.deadline
                        ? new Date(b.deadline).getTime()
                        : Number.MAX_SAFE_INTEGER;
                      return ta - tb;
                    })
                    .map((t) => {
                      const deadline = (() => {
                        const formatted = t.deadline
                          ? formatDateYMDSlash(t.deadline)
                          : null;
                        const label = deriveLabelFromEnums(
                          t.deadline || "",
                          t.deadline_since,
                          t.deadline_until
                        );
                        if (formatted) {
                          return label ? `${formatted} ${label}` : formatted;
                        }
                        return "期限なし";
                      })();
                      const isEditing = editingId === t.id;
                      return (
                        <li
                          key={t.id}
                          className="rounded-lg border p-4 bg-card text-card-foreground"
                        >
                          {isEditing ? (
                            <div className="space-y-3">
                              <div>
                                <Label htmlFor={`ed-title-${t.id}`}>
                                  タイトル
                                </Label>
                                <Input
                                  id={`ed-title-${t.id}`}
                                  value={editTitle}
                                  onChange={(e) => setEditTitle(e.target.value)}
                                />
                              </div>
                              <div className="grid grid-cols-2 gap-3">
                                <div>
                                  <Label htmlFor={`ed-subject-${t.id}`}>
                                    サブジェクト
                                  </Label>
                                  <SubjectSelect
                                    value={editSubjectId}
                                    onChange={setEditSubjectId}
                                    subjects={subjects}
                                    placeholder="選択してください"
                                  />
                                </div>
                                <div>
                                  <Label htmlFor={`ed-date-${t.id}`}>
                                    期限
                                  </Label>
                                  <DeadlinePicker
                                    date={editDate}
                                    meta={editMeta}
                                    onChange={(d, m) => {
                                      setEditDate(d);
                                      setEditMeta(m);
                                    }}
                                  />
                                </div>
                              </div>
                              <div>
                                <Label>タイプ</Label>
                                <Select
                                  value={editType}
                                  onValueChange={(v: string) => setEditType(v)}
                                >
                                  <SelectTrigger>
                                    <SelectValue placeholder="選択してください" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="オンライン">
                                      オンライン
                                    </SelectItem>
                                    <SelectItem value="対面">対面</SelectItem>
                                    <SelectItem value="小テスト">
                                      小テスト
                                    </SelectItem>
                                    <SelectItem value="発表">発表</SelectItem>
                                    <SelectItem value="自習/予習">
                                      自習/予習
                                    </SelectItem>
                                    <SelectItem value="その他">
                                      その他
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>
                              <div>
                                <Label>説明（任意）</Label>
                                <textarea
                                  className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                                  rows={3}
                                  value={editDesc}
                                  onChange={(e) => setEditDesc(e.target.value)}
                                />
                              </div>
                              <div className="flex items-center gap-2">
                                <Switch
                                  id={`ed-req-${t.id}`}
                                  checked={editRequire}
                                  onCheckedChange={setEditRequire}
                                />
                                <Label htmlFor={`ed-req-${t.id}`}>
                                  提出必須
                                </Label>
                              </div>
                              <div className="flex items-center gap-2 -mt-2">
                                <Switch
                                  checked={editSelfCheck}
                                  onCheckedChange={setEditSelfCheck}
                                />
                                <Label>自己採点が必要</Label>
                              </div>
                              <div className="flex gap-2">
                                <Button
                                  size="sm"
                                  onClick={() => saveEdit(t.id)}
                                >
                                  <Save className="h-4 w-4 mr-1" /> 保存
                                </Button>
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  onClick={cancelEdit}
                                >
                                  <X className="h-4 w-4 mr-1" /> キャンセル
                                </Button>
                              </div>
                            </div>
                          ) : (
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <div className="mb-1 flex flex-wrap gap-1">
                                  {t.type && (
                                    <Badge variant="secondary">{t.type}</Badge>
                                  )}
                                  {t.is_require_hand_in && (
                                    <Badge variant="destructive">
                                      提出必須
                                    </Badge>
                                  )}
                                  {t.self_check_required && (
                                    <Badge>自己採点</Badge>
                                  )}
                                </div>
                                <div className="font-medium">{t.title}</div>
                                {(() => {
                                  const subj = subjects.find(
                                    (s) => s.id === t.subject_id
                                  );
                                  const subjectTitle = subj
                                    ? subj.title
                                    : `教科ID: ${t.subject_id}`;
                                  return (
                                    <div className="text-sm text-muted-foreground">
                                      {subjectTitle} {deadline}
                                    </div>
                                  );
                                })()}
                                {t.description && (
                                  <p
                                    className="text-sm text-muted-foreground mt-1"
                                    style={{
                                      display: "-webkit-box",
                                      WebkitLineClamp: 3,
                                      WebkitBoxOrient: "vertical",
                                      overflow: "hidden",
                                    }}
                                  >
                                    {t.description}
                                  </p>
                                )}
                              </div>
                              <div className="flex items-center gap-2">
                                {/* Desktop: grouped buttons */}
                                <ButtonGroup className="hidden sm:flex">
                                  <Button
                                    size="sm"
                                    variant="secondary"
                                    onClick={() => beginEdit(t)}
                                  >
                                    <Pencil className="h-4 w-4 mr-1" /> 編集
                                  </Button>
                                  <ButtonGroupSeparator />
                                  <Button
                                    size="sm"
                                    variant="destructive"
                                    onClick={() => deleteTask(t.id)}
                                  >
                                    <Trash2 className="h-4 w-4 mr-1" /> 削除
                                  </Button>
                                </ButtonGroup>
                                {/* Mobile: kebab menu */}
                                <div className="sm:hidden">
                                  <DropdownMenu>
                                    <DropdownMenuTrigger asChild>
                                      <Button
                                        size="icon"
                                        variant="ghost"
                                        aria-label="アクション"
                                      >
                                        <MoreVertical className="h-4 w-4" />
                                      </Button>
                                    </DropdownMenuTrigger>
                                    <DropdownMenuContent align="end">
                                      <DropdownMenuItem
                                        onClick={() => beginEdit(t)}
                                      >
                                        <Pencil className="h-4 w-4 mr-2" /> 編集
                                      </DropdownMenuItem>
                                      <DropdownMenuItem
                                        className="text-destructive focus:text-destructive"
                                        onClick={() => deleteTask(t.id)}
                                      >
                                        <Trash2 className="h-4 w-4 mr-2" /> 削除
                                      </DropdownMenuItem>
                                    </DropdownMenuContent>
                                  </DropdownMenu>
                                </div>
                              </div>
                            </div>
                          )}
                        </li>
                      );
                    })}
                </ul>
              </div>
            ))}
          </div>
        );
      })()}
    </div>
  );
}
