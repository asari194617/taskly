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
import { Skeleton } from "@/components/ui/skeleton";
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
import { useMediaQuery } from "@/react/hooks/useMediaQuery";
import {
  DeadlinePicker,
  type DeadlineMeta,
  sinceUntilFromMeta,
  deriveLabelFromEnums,
} from "@/components/DeadlinePicker";
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
  ButtonGroup,
  ButtonGroupSeparator,
} from "@/components/ui/button-group";
import {
  AlertCircle,
  Hourglass,
  FileText,
  Plus,
  Pencil,
  Trash2,
  Save,
  X,
  MoreVertical,
  Search,
} from "lucide-react";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { t, type Lang } from "@/lib/i18n";

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
type Subject = { id: number; name_ja: string; name_en?: string | null };

// Slot-based helpers removed; we now use symbolic meta via DeadlinePicker

// RFC3339 helpers removed — we now carry date string directly

// slotFromRFC3339 removed

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function formatDateYMDSlash(s: string): string {
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/);
  return m ? `${m[1]}/${m[2]}/${m[3]}` : s;
}

function localizeTaskType(
  type: string | null | undefined,
  dict: ReturnType<typeof t>["tasks"]
): string {
  if (!type) return "";
  const upper = type.toUpperCase();
  switch (upper) {
    case "ONLINE":
      return dict.typeOnline;
    case "FACE_TO_FACE":
      return dict.typeFaceToFace;
    case "QUIZ":
      return dict.typeQuiz;
    case "PRESENTATION":
      return dict.typePresentation;
    case "SELF_STUDY":
      return dict.typeSelfStudy;
    case "OTHER":
      return dict.typeOther;
    default:
      return type;
  }
}

export default function TasksList() {
  // Language management
  const [lang, setLang] = React.useState<Lang>("ja");
  React.useEffect(() => {
    const updateLang = () => {
      const stored = localStorage.getItem("display_lang") as Lang | null;
      setLang(stored || "ja");
    };
    updateLang();
    window.addEventListener("languagechange", updateLang);
    window.addEventListener("storage", (e) => {
      if (e.key === "display_lang") updateLang();
    });
    return () => {
      window.removeEventListener("languagechange", updateLang);
    };
  }, []);
  const dict = React.useMemo(() => t(lang), [lang]);

  const formatHeadingForDate = React.useCallback(
    (d: Date): string => {
      const today = startOfDay(new Date());
      const diffMs = startOfDay(d).getTime() - today.getTime();
      const diffDays = Math.round(diffMs / (1000 * 60 * 60 * 24));
      if (diffDays === 0) return dict.tasks.today;
      if (diffDays === 1) return dict.tasks.tomorrow;
      if (diffDays === 2) return dict.tasks.dayAfterTomorrow;
      const y = d.getFullYear();
      const m = String(d.getMonth() + 1).padStart(2, "0");
      const day = String(d.getDate()).padStart(2, "0");
      return `${y}/${m}/${day}`;
    },
    [dict.tasks]
  );

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
        `query($q:String){ tasks(limit:200, q:$q){ items{ id title subject_id deadline deadline_since deadline_until description type is_require_hand_in self_check_required } } }`,
        { q: q || undefined }
      ),
      gql<{ subjects: Subject[] }>(`query{ subjects{ id name_ja name_en } }`),
    ])
      .then(([t, s]) => {
        setItems(t.tasks.items);
        setSubjects(s.subjects || []);
      })
      .catch((e) => setError(e?.message || dict.tasks.loadFailed));
  }, [q, dict.tasks.loadFailed]);

  // debounce search input changes
  const [qInput, setQInput] = React.useState("");
  React.useEffect(() => {
    const id = setTimeout(() => setQ(qInput.trim()), 300);
    return () => clearTimeout(id);
  }, [qInput]);

  // Items to show: by default hide past tasks (deadline before today), but include all when searching
  const shownItems = React.useMemo(() => {
    if (!items) return [] as Task[];
    const searching = q.trim().length > 0;
    if (searching) return items;
    const todayStart = startOfDay(new Date()).getTime();
    return items.filter((t) => {
      if (!t.deadline) return true;
      const d = new Date(t.deadline);
      const sod = startOfDay(d).getTime();
      return sod >= todayStart;
    });
  }, [items, q]);

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
          title: dict.tasks.shortcutsTitle,
          description: dict.tasks.shortcutsDesc,
        });
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [dict.tasks]);

  // Ctrl+S / Cmd+S to save when editing
  React.useEffect(() => {
    function onSaveKey(e: KeyboardEvent) {
      const isSave =
        (e.key === "s" || e.key === "S") && (e.metaKey || e.ctrlKey);
      if (!isSave) return;
      if (editingId != null) {
        e.preventDefault();
        // Avoid duplicate submissions if already saving could be added here
        saveEdit(editingId);
      }
    }
    window.addEventListener("keydown", onSaveKey);
    return () => window.removeEventListener("keydown", onSaveKey);
  }, [
    editingId,
    editTitle,
    editSubjectId,
    editDate,
    editMeta,
    editRequire,
    editSelfCheck,
    editType,
    editDesc,
  ]);

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
    const filtered = subjects.filter((s) => {
      const name = lang === "en" && s.name_en ? s.name_en : s.name_ja;
      return name.toLowerCase().includes(filter.toLowerCase());
    });
    const displayName = (s: Subject) =>
      lang === "en" && s.name_en ? s.name_en : s.name_ja;
    return (
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            type="button"
            className="w-full justify-between"
          >
            <span className="truncate">
              {selected
                ? displayName(selected)
                : placeholder || dict.tasks.selectPlaceholder}
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
              placeholder={dict.common.search}
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
                {displayName(s)}
              </DropdownMenuItem>
            ))}
            {filtered.length === 0 && (
              <div className="px-2 py-6 text-center text-sm text-muted-foreground">
                {dict.tasks.notFound}
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
        title: dict.toast.requiredFields,
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
      toast({ title: dict.toast.created });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.createFailed,
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
      toast({ title: dict.toast.updated });
      cancelEdit();
    } catch (e: any) {
      // revert
      setItems(prev);
      toast({
        variant: "destructive",
        title: dict.toast.updateFailed,
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
      toast({ title: dict.toast.deleted });
    } catch (e: any) {
      setItems(prev);
      toast({
        variant: "destructive",
        title: dict.toast.deleteFailed,
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
          <EmptyTitle>{dict.tasks.loadFailed}</EmptyTitle>
          <EmptyDescription>{error}</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button onClick={() => window.location.reload()}>
            {dict.common.reload}
          </Button>
        </EmptyContent>
      </Empty>
    );
  }

  if (!items) {
    return (
      <div className="space-y-3">
        {[1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="border rounded-lg p-4 space-y-3">
            <div className="flex items-start justify-between">
              <div className="space-y-2 flex-1">
                <Skeleton className="h-5 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
              </div>
              <Skeleton className="h-6 w-16" />
            </div>
            <Skeleton className="h-4 w-full" />
          </div>
        ))}
      </div>
    );
  }

  // items が空のときに下部で適切な Empty を出す（ツールバーは常に表示するため early return しない）

  return (
    <div className="space-y-6">
      {/* Toolbar */}
      <div className="flex items-center justify-end gap-2">
        <div className="flex-1">
          <InputGroup>
            <InputGroupInput
              placeholder={dict.tasks.searchPlaceholder}
              value={qInput}
              onChange={(e) => setQInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  setQ(qInput.trim());
                }
              }}
            />
            <InputGroupAddon>
              <Search className="h-4 w-4" />
            </InputGroupAddon>
            <InputGroupAddon align="inline-end">
              {shownItems.length} {dict.tasks.itemsCount}
            </InputGroupAddon>
          </InputGroup>
        </div>
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="sm"
                onClick={() => setCreateOpen(true)}
                className="sm:inline-flex hidden"
              >
                <Plus className="h-4 w-4 mr-1" /> {dict.tasks.newTask}
              </Button>
            </TooltipTrigger>
            <TooltipContent>{dict.tasks.createNewTask}</TooltipContent>
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

      {/* New Task Form (responsive dialog/drawer) */}
      <Dialog
        open={isDesktop && createOpen}
        onOpenChange={(v: boolean) => setCreateOpen(isDesktop ? v : createOpen)}
      >
        <DialogContent className="sm:max-w-[640px]">
          <DialogHeader>
            <DialogTitle>{dict.tasks.createTaskTitle}</DialogTitle>
            <DialogDescription>
              {dict.tasks.enterRequiredInfo}
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3">
            <div className="col-span-2">
              <Label htmlFor="nt-title-2">{dict.tasks.title}</Label>
              <Input
                id="nt-title-2"
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                placeholder={dict.tasks.titlePlaceholder}
              />
            </div>
            <div>
              <Label htmlFor="nt-subject-2">{dict.tasks.subject}</Label>
              <SubjectSelect
                value={newSubjectId}
                onChange={setNewSubjectId}
                subjects={subjects}
              />
            </div>
            <div>
              <Label htmlFor="nt-date-2">{dict.tasks.deadline}</Label>
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
              <Label htmlFor="nt-req-2">{dict.tasks.requireHandIn}</Label>
            </div>
            <div className="col-span-2 flex items-center gap-2 -mt-2">
              <Switch
                id="nt-scr-2"
                checked={newSelfCheck}
                onCheckedChange={setNewSelfCheck}
              />
              <Label htmlFor="nt-scr-2">{dict.tasks.selfCheckRequired}</Label>
            </div>
            <div>
              <Label htmlFor="nt-type-2">{dict.tasks.type}</Label>
              <Select
                value={newType}
                onValueChange={(v: string) => setNewType(v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={dict.tasks.selectPlaceholder} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={dict.tasks.typeOnline}>
                    {dict.tasks.typeOnline}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeFaceToFace}>
                    {dict.tasks.typeFaceToFace}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeQuiz}>
                    {dict.tasks.typeQuiz}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typePresentation}>
                    {dict.tasks.typePresentation}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeSelfStudy}>
                    {dict.tasks.typeSelfStudy}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeOther}>
                    {dict.tasks.typeOther}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="col-span-2 mt-0">
              <Label htmlFor="nt-desc-2">
                {dict.tasks.description} {dict.tasks.optional}
              </Label>
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
              {dict.tasks.create}
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
            <DrawerTitle>{dict.tasks.createTaskTitle}</DrawerTitle>
            <DrawerDescription>
              {dict.tasks.enterRequiredInfo}
            </DrawerDescription>
          </DrawerHeader>
          <div className="px-4 pb-4 grid grid-cols-2 gap-3">
            {/* form fields are the same as dialog */}
            <div className="col-span-2">
              <Label>{dict.tasks.title}</Label>
              <Input
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                placeholder={dict.tasks.titlePlaceholder}
              />
            </div>
            <div>
              <Label>{dict.tasks.subject}</Label>
              <SubjectSelect
                value={newSubjectId}
                onChange={setNewSubjectId}
                subjects={subjects}
              />
            </div>
            <div>
              <Label>{dict.tasks.deadline}</Label>
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
              <Label htmlFor="nt-req-m">{dict.tasks.requireHandIn}</Label>
            </div>
            <div className="col-span-2 flex items-center gap-2 -mt-2">
              <Switch
                id="nt-scr-m"
                checked={newSelfCheck}
                onCheckedChange={setNewSelfCheck}
              />
              <Label htmlFor="nt-scr-m">{dict.tasks.selfCheckRequired}</Label>
            </div>
            <div>
              <Label>{dict.tasks.type}</Label>
              <Select
                value={newType}
                onValueChange={(v: string) => setNewType(v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={dict.tasks.selectPlaceholder} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={dict.tasks.typeOnline}>
                    {dict.tasks.typeOnline}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeFaceToFace}>
                    {dict.tasks.typeFaceToFace}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeQuiz}>
                    {dict.tasks.typeQuiz}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typePresentation}>
                    {dict.tasks.typePresentation}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeSelfStudy}>
                    {dict.tasks.typeSelfStudy}
                  </SelectItem>
                  <SelectItem value={dict.tasks.typeOther}>
                    {dict.tasks.typeOther}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="col-span-2">
              <Label>
                {dict.tasks.description} {dict.tasks.optional}
              </Label>
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
                {dict.tasks.create}
              </Button>
            </div>
          </div>
        </DrawerContent>
      </Drawer>
      {/* Empty states below toolbar */}
      {shownItems.length === 0 &&
        (q ? (
          <Empty className="border">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <FileText className="h-5 w-5" />
              </EmptyMedia>
              <EmptyTitle>{dict.tasks.noSearchResults}</EmptyTitle>
              <EmptyDescription>
                {dict.tasks.noSearchResultsDesc}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <Empty className="border">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <FileText className="h-5 w-5" />
              </EmptyMedia>
              <EmptyTitle>{dict.tasks.noTasks}</EmptyTitle>
              <EmptyDescription>{dict.tasks.noTasksDesc}</EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <div className="w-full max-w-xl text-left space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div className="col-span-2">
                    <Label htmlFor="nt-title">{dict.tasks.title}</Label>
                    <Input
                      id="nt-title"
                      value={newTitle}
                      onChange={(e) => setNewTitle(e.target.value)}
                      placeholder={dict.tasks.titlePlaceholder}
                    />
                  </div>
                  <div>
                    <Label htmlFor="nt-subject">{dict.tasks.subject}</Label>
                    <SubjectSelect
                      value={newSubjectId}
                      onChange={setNewSubjectId}
                      subjects={subjects}
                    />
                  </div>
                  <div>
                    <Label htmlFor="nt-date">{dict.tasks.deadline}</Label>
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
                    <Label htmlFor="nt-req">{dict.tasks.requireHandIn}</Label>
                  </div>
                  <div className="col-span-2 flex items-center gap-2 -mt-2">
                    <Switch
                      id="nt-scr"
                      checked={newSelfCheck}
                      onCheckedChange={setNewSelfCheck}
                    />
                    <Label htmlFor="nt-scr">
                      {dict.tasks.selfCheckRequired}
                    </Label>
                  </div>
                  <div>
                    <Label htmlFor="nt-type">{dict.tasks.type}</Label>
                    <Select
                      value={newType}
                      onValueChange={(v: string) => setNewType(v)}
                    >
                      <SelectTrigger>
                        <SelectValue
                          placeholder={dict.tasks.selectPlaceholder}
                        />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={dict.tasks.typeOnline}>
                          {dict.tasks.typeOnline}
                        </SelectItem>
                        <SelectItem value={dict.tasks.typeFaceToFace}>
                          {dict.tasks.typeFaceToFace}
                        </SelectItem>
                        <SelectItem value={dict.tasks.typeQuiz}>
                          {dict.tasks.typeQuiz}
                        </SelectItem>
                        <SelectItem value={dict.tasks.typePresentation}>
                          {dict.tasks.typePresentation}
                        </SelectItem>
                        <SelectItem value={dict.tasks.typeSelfStudy}>
                          {dict.tasks.typeSelfStudy}
                        </SelectItem>
                        <SelectItem value={dict.tasks.typeOther}>
                          {dict.tasks.typeOther}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="col-span-2 mt-0">
                    <Label htmlFor="nt-desc">
                      {dict.tasks.description} {dict.tasks.optional}
                    </Label>
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
                  {dict.tasks.create}
                </Button>
              </div>
            </EmptyContent>
          </Empty>
        ))}

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
        for (const t of shownItems) {
          if (!t.deadline) {
            const key = dict.tasks.noDeadline;
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
                        if (!t.deadline) return dict.tasks.noDeadline;
                        const dateStr = formatDateYMDSlash(t.deadline);
                        const label = deriveLabelFromEnums(
                          t.deadline,
                          t.deadline_since,
                          t.deadline_until,
                          lang
                        );
                        return label ? `${dateStr} ${label}` : dateStr;
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
                                  {dict.tasks.title}
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
                                    {dict.tasks.subject}
                                  </Label>
                                  <SubjectSelect
                                    value={editSubjectId}
                                    onChange={setEditSubjectId}
                                    subjects={subjects}
                                  />
                                </div>
                                <div>
                                  <Label htmlFor={`ed-date-${t.id}`}>
                                    {dict.tasks.deadline}
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
                                <Label>{dict.tasks.type}</Label>
                                <Select
                                  value={editType}
                                  onValueChange={(v: string) => setEditType(v)}
                                >
                                  <SelectTrigger>
                                    <SelectValue
                                      placeholder={dict.tasks.selectPlaceholder}
                                    />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value={dict.tasks.typeOnline}>
                                      {dict.tasks.typeOnline}
                                    </SelectItem>
                                    <SelectItem
                                      value={dict.tasks.typeFaceToFace}
                                    >
                                      {dict.tasks.typeFaceToFace}
                                    </SelectItem>
                                    <SelectItem value={dict.tasks.typeQuiz}>
                                      {dict.tasks.typeQuiz}
                                    </SelectItem>
                                    <SelectItem
                                      value={dict.tasks.typePresentation}
                                    >
                                      {dict.tasks.typePresentation}
                                    </SelectItem>
                                    <SelectItem
                                      value={dict.tasks.typeSelfStudy}
                                    >
                                      {dict.tasks.typeSelfStudy}
                                    </SelectItem>
                                    <SelectItem value={dict.tasks.typeOther}>
                                      {dict.tasks.typeOther}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>
                              <div>
                                <Label>
                                  {dict.tasks.description} {dict.tasks.optional}
                                </Label>
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
                                  {dict.tasks.requireHandIn}
                                </Label>
                              </div>
                              <div className="flex items-center gap-2 -mt-2">
                                <Switch
                                  checked={editSelfCheck}
                                  onCheckedChange={setEditSelfCheck}
                                />
                                <Label>{dict.tasks.selfCheckRequired}</Label>
                              </div>
                              <div className="flex gap-2">
                                <Button
                                  size="sm"
                                  onClick={() => saveEdit(t.id)}
                                >
                                  <Save className="h-4 w-4 mr-1" />{" "}
                                  {dict.tasks.save}
                                </Button>
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  onClick={cancelEdit}
                                >
                                  <X className="h-4 w-4 mr-1" />{" "}
                                  {dict.tasks.cancel}
                                </Button>
                              </div>
                            </div>
                          ) : (
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <div className="mb-1 flex flex-wrap gap-1">
                                  {t.type && (
                                    <Badge variant="secondary">
                                      {localizeTaskType(t.type, dict.tasks)}
                                    </Badge>
                                  )}
                                  {t.is_require_hand_in && (
                                    <Badge variant="destructive">
                                      {dict.tasks.requireHandIn}
                                    </Badge>
                                  )}
                                  {t.self_check_required && (
                                    <Badge>{dict.tasks.selfCheckBadge}</Badge>
                                  )}
                                </div>
                                <div className="font-medium">{t.title}</div>
                                {(() => {
                                  const subj = subjects.find(
                                    (s) => s.id === t.subject_id
                                  );
                                  const subjectTitle = subj
                                    ? lang === "en" && subj.name_en
                                      ? subj.name_en
                                      : subj.name_ja
                                    : `${dict.tasks.subjectId}: ${t.subject_id}`;
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
                                    <Pencil className="h-4 w-4 mr-1" />{" "}
                                    {dict.tasks.edit}
                                  </Button>
                                  <ButtonGroupSeparator />
                                  <Button
                                    size="sm"
                                    variant="destructive"
                                    onClick={() => deleteTask(t.id)}
                                  >
                                    <Trash2 className="h-4 w-4 mr-1" />{" "}
                                    {dict.tasks.delete}
                                  </Button>
                                </ButtonGroup>
                                {/* Mobile: kebab menu */}
                                <div className="sm:hidden">
                                  <DropdownMenu>
                                    <DropdownMenuTrigger asChild>
                                      <Button
                                        size="icon"
                                        variant="ghost"
                                        aria-label={dict.tasks.actions}
                                      >
                                        <MoreVertical className="h-4 w-4" />
                                      </Button>
                                    </DropdownMenuTrigger>
                                    <DropdownMenuContent align="end">
                                      <DropdownMenuItem
                                        onClick={() => beginEdit(t)}
                                      >
                                        <Pencil className="h-4 w-4 mr-2" />{" "}
                                        {dict.tasks.edit}
                                      </DropdownMenuItem>
                                      <DropdownMenuItem
                                        className="text-destructive focus:text-destructive"
                                        onClick={() => deleteTask(t.id)}
                                      >
                                        <Trash2 className="h-4 w-4 mr-2" />{" "}
                                        {dict.tasks.delete}
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
