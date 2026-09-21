import * as React from "react";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
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
  DialogFooter,
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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
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
import { t } from "@/lib/i18n";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { useLang } from "@/react/LanguageProvider";
import { SubjectSelect as SubjectSelectControl } from "@/features/tasks/SubjectSelect";
import {
  createTask as createTaskRequest,
  deleteTask as deleteTaskRequest,
  loadSubjects,
  loadViewerRole,
  reportTask,
  searchTasks,
  setTaskCompleted,
  updateTask as updateTaskRequest,
  type Subject,
  type Task,
} from "@/features/tasks/api";
import {
  normalizeTaskSearch,
  updateTaskCompletion,
} from "@/features/tasks/state.mjs";
import { getToken, requireAuth } from "@/lib/auth-guard";

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
  const { lang, dict } = useLang();

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
  const [isAdmin, setIsAdmin] = React.useState(false);
  const [isManager, setIsManager] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [q, setQ] = React.useState("");
  const [completedFilter, setCompletedFilter] = React.useState<
    "incomplete" | "completed" | "all"
  >("all");
  // Pending completion/uncompletion operations with undo window
  const [pending, setPending] = React.useState<
    Record<number, { target: boolean; timeout: ReturnType<typeof setTimeout> }>
  >({});
  const pendingTimers = React.useRef(new Set<ReturnType<typeof setTimeout>>());
  React.useEffect(
    () => () => {
      for (const timer of pendingTimers.current) clearTimeout(timer);
      pendingTimers.current.clear();
    },
    [],
  );
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
  // One-time bootstrap: subjects and my role; avoid reloading on each search
  React.useEffect(() => {
    if (!getToken()) {
      requireAuth();
      return;
    }
    loadSubjects()
      .then(setSubjects)
      .catch((e) => setError(e?.message || dict.tasks.loadFailed));
    loadViewerRole()
      .then((role) => {
        setIsAdmin(role.isAdmin);
        setIsManager(role.isManager);
      })
      .catch(() => {
        /* ignore role load error for now */
      });
  }, [dict.tasks.loadFailed]);

  // Search tasks with cancellation; only refetch when q changes
  React.useEffect(() => {
    if (!getToken()) {
      requireAuth();
      return;
    }
    const ac = new AbortController();
    (async () => {
      try {
        setItems(await searchTasks(q, ac.signal));
      } catch (e: any) {
        if (e?.name === "AbortError") return;
        setError(e?.message || dict.tasks.loadFailed);
      }
    })();
    return () => ac.abort();
  }, [q, dict.tasks.loadFailed]);

  // debounce search input changes
  const [qInput, setQInput] = React.useState("");
  React.useEffect(() => {
    const id = setTimeout(() => {
      setQ(normalizeTaskSearch(qInput));
    }, 500);
    return () => clearTimeout(id);
  }, [qInput]);

  // Items to show: by default hide past tasks (deadline before today), but include all when searching
  const shownItems = React.useMemo(() => {
    if (!items) return [] as Task[];
    const searching = q.trim().length > 0;
    let base = items;
    // Apply completion filter
    if (completedFilter === "incomplete")
      base = base.filter((t) => !t.is_completed);
    else if (completedFilter === "completed")
      base = base.filter((t) => !!t.is_completed);
    if (searching) return base;
    const todayStart = startOfDay(new Date()).getTime();
    return base.filter((t) => {
      if (!t.deadline) return true;
      const d = new Date(t.deadline);
      const sod = startOfDay(d).getTime();
      return sod >= todayStart;
    });
  }, [items, q, completedFilter]);

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
    return (
      <SubjectSelectControl
        {...props}
        language={lang}
        searchLabel={dict.common.search}
        emptyLabel={dict.tasks.notFound}
        placeholder={props.placeholder || dict.tasks.selectPlaceholder}
      />
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
      const created = await createTaskRequest({
          title: newTitle.trim(),
          subject_id: parseInt(newSubjectId, 10),
          is: newRequire,
          scr: newSelfCheck,
          deadline: newDate,
          ...sinceUntilFromMeta(newDate, newMeta), // enums
          description: newDesc || undefined,
          type: newType || undefined,
        });
      setItems((prev) => (prev ? [created, ...prev] : [created]));
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
    setEditSelfCheck(!!t.is_self_check_required);
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
              is_self_check_required: editSelfCheck,
              type: editType,
              description: editDesc,
            }
          : x
      )
    );
    try {
      await updateTaskRequest(id, {
          title: editTitle,
          subject_id: editSubjectId ? parseInt(editSubjectId, 10) : undefined,
          is: editRequire,
          scr: editSelfCheck,
          deadline: editDate || undefined,
          ...sinceUntilFromMeta(editDate, editMeta),
          description: editDesc,
          type: editType,
        });
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
      await deleteTaskRequest(id);
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

  // Toggle completion with pending undo window
  function toggleWithPending(task: Task) {
    // optimistic flip state for visual feedback
    setItems((cur) =>
      updateTaskCompletion(cur || [], task.id, !task.is_completed)
    );
    const existing = pending[task.id];
    if (existing) {
      // Undo: cancel timer and revert state
      clearTimeout(existing.timeout);
      pendingTimers.current.delete(existing.timeout);
      setPending((p) => {
        const cp = { ...p };
        delete cp[task.id];
        return cp;
      });
      setItems((cur) =>
        updateTaskCompletion(cur || [], task.id, !!task.is_completed)
      );
      return;
    }
    const target = !task.is_completed;
    const timeout = setTimeout(async () => {
      pendingTimers.current.delete(timeout);
      setPending((p) => {
        const cp = { ...p };
        delete cp[task.id];
        return cp;
      });
      try {
        await setTaskCompleted(task.id, target);
      } catch (err: any) {
        // revert on error
        setItems((cur) =>
          updateTaskCompletion(cur || [], task.id, !target)
        );
        toast({
          variant: "destructive",
          title: dict.toast.updateFailed,
          description: err?.message,
        });
      }
    }, 3000);
    pendingTimers.current.add(timeout);
    setPending((p) => ({ ...p, [task.id]: { target, timeout } }));
  }

  // Delete confirmation dialog state
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteTargetId, setDeleteTargetId] = React.useState<number | null>(
    null
  );
  // Report dialog state
  const [reportOpen, setReportOpen] = React.useState(false);
  const [reportTargetId, setReportTargetId] = React.useState<number | null>(
    null
  );
  // Report form state (within action popover)
  const [reportReason, setReportReason] = React.useState<string>("");
  const [reportComment, setReportComment] = React.useState<string>("");
  const [reportSubmittingId, setReportSubmittingId] = React.useState<
    number | null
  >(null);

  async function submitReport(taskId: number) {
    if (!reportReason.trim()) {
      toast({ title: dict.toast.requiredFields });
      return;
    }
    try {
      setReportSubmittingId(taskId);
      await reportTask(
        taskId,
        reportReason.trim(),
        reportComment.trim() || undefined,
      );
      toast({ title: dict.toast.created });
      setReportReason("");
      setReportComment("");
    } catch (e: any) {
      toast({ title: dict.toast.createFailed, description: e?.message || "" });
    } finally {
      setReportSubmittingId(null);
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
                  setQ(normalizeTaskSearch(qInput));
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
        {/* Removed select filter (moved to badges) */}
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

      {/* Completion filter badges ("すべて" omitted). Click active badge to clear filter. */}
      <div className="flex flex-wrap gap-2 -mt-2">
        {[
          { key: "incomplete", label: "未完了" },
          { key: "completed", label: "完了済み" },
        ].map((f) => {
          const active = completedFilter === f.key;
          return (
            <Badge
              key={f.key}
              onClick={() =>
                setCompletedFilter((prev) =>
                  prev === f.key ? "all" : (f.key as any)
                )
              }
              className={
                "cursor-pointer select-none px-3 py-1" +
                (active
                  ? " bg-primary text-primary-foreground"
                  : " bg-muted text-muted-foreground")
              }
            >
              {f.label}
            </Badge>
          );
        })}
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
        ) : completedFilter !== "all" ? (
          <Empty className="border">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <FileText className="h-5 w-5" />
              </EmptyMedia>
              <EmptyTitle>{dict.tasks.noTasks}</EmptyTitle>
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
                      const pendingInfo = pending[t.id];
                      return (
                        <li
                          key={t.id}
                          onClick={(e) => {
                            const tag = (e.target as HTMLElement).tagName;
                            if (
                              [
                                "BUTTON",
                                "INPUT",
                                "TEXTAREA",
                                "LABEL",
                                "SELECT",
                                "SVG",
                                "PATH",
                              ].includes(tag)
                            )
                              return;
                            if (isEditing) return;
                            toggleWithPending(t);
                          }}
                          className={
                            "relative rounded-lg border p-4 pl-12 bg-card text-card-foreground transition-opacity cursor-pointer " +
                            (pendingInfo ? " opacity-50" : "")
                          }
                        >
                          {/* Radio vertically centered on left */}
                          <RadioGroup
                            value={t.is_completed ? "done" : "undone"}
                            onValueChange={(val: string) => {
                              const target = val === "done";
                              if (target !== !!t.is_completed) {
                                toggleWithPending(t);
                              }
                            }}
                            className="absolute left-4 top-1/2 -translate-y-1/2"
                          >
                            <RadioGroupItem
                              value="undone"
                              className="sr-only"
                              aria-label="未完了"
                            />
                            <RadioGroupItem value="done" aria-label="完了" />
                          </RadioGroup>
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
                                  {t.is_self_check_required && (
                                    <Badge>{dict.tasks.selfCheckBadge}</Badge>
                                  )}
                                  {(isAdmin || isManager) && t.is_hidden && (
                                    <Badge variant="outline">非表示</Badge>
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
                                {/* Unified actions via Popover across sizes */}
                                <Popover>
                                  <PopoverTrigger asChild>
                                    <Button
                                      size="icon"
                                      variant="ghost"
                                      aria-label={dict.tasks.actions}
                                    >
                                      <MoreVertical className="h-4 w-4" />
                                    </Button>
                                  </PopoverTrigger>
                                  <PopoverContent className="w-64" align="end">
                                    <div className="flex flex-col gap-2">
                                      <Button
                                        size="sm"
                                        variant="secondary"
                                        onClick={() => beginEdit(t)}
                                      >
                                        <Pencil className="h-4 w-4 mr-1" />{" "}
                                        {dict.tasks.edit}
                                      </Button>
                                      {(isAdmin || isManager) && (
                                        <Button
                                          size="sm"
                                          variant="destructive"
                                          onClick={() => {
                                            setDeleteTargetId(t.id);
                                            setDeleteOpen(true);
                                          }}
                                        >
                                          <Trash2 className="h-4 w-4 mr-1" />{" "}
                                          {dict.tasks.delete}
                                        </Button>
                                      )}
                                      <div className="pt-2 border-t">
                                        <Button
                                          className="mt-2"
                                          size="sm"
                                          onClick={() => {
                                            setReportTargetId(t.id);
                                            setReportOpen(true);
                                          }}
                                        >
                                          {dict.tasks.reportSubmit}
                                        </Button>
                                      </div>
                                    </div>
                                  </PopoverContent>
                                </Popover>
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
      {/* Delete confirm dialog */}
      <TasksListDeleteDialog
        open={deleteOpen}
        setOpen={setDeleteOpen}
        onConfirm={() => {
          if (deleteTargetId != null) {
            deleteTask(deleteTargetId);
            setDeleteTargetId(null);
          }
        }}
      />
      {/* Report dialog */}
      <Dialog open={reportOpen} onOpenChange={setReportOpen}>
        <DialogContent className="sm:max-w-[520px]">
          <DialogHeader>
            <DialogTitle>{dict.tasks.reportSubmit}</DialogTitle>
            <DialogDescription>
              {dict.tasks.enterRequiredInfo}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="report-reason">
                {dict.tasks.reportReasonLabel ?? "Reason"}
              </Label>
              <Input
                id="report-reason"
                value={reportReason}
                onChange={(e) => setReportReason(e.target.value)}
                placeholder={
                  dict.tasks.reportReasonPlaceholder ??
                  "Spam/Inappropriate/etc."
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="report-comment">
                {dict.tasks.reportCommentLabel ?? "Comment"}
              </Label>
              <Input
                id="report-comment"
                value={reportComment}
                onChange={(e) => setReportComment(e.target.value)}
                placeholder={
                  dict.tasks.reportCommentPlaceholder ?? "Optional details"
                }
              />
            </div>
            <DialogFooter>
              <Button
                onClick={() => {
                  if (reportTargetId != null) {
                    submitReport(reportTargetId);
                  }
                }}
                disabled={
                  reportTargetId != null &&
                  reportSubmittingId === reportTargetId
                }
              >
                {dict.tasks.reportSubmit}
              </Button>
            </DialogFooter>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// Delete confirm dialog mounted at root of TasksList
// Note: placed after component to keep JSX readable
export function TasksListDeleteDialog({
  open,
  setOpen,
  onConfirm,
}: {
  open: boolean;
  setOpen: (v: boolean) => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>削除の確認</DialogTitle>
          <DialogDescription>
            この課題を削除します。よろしいですか？
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            キャンセル
          </Button>
          <Button
            variant="destructive"
            onClick={() => {
              onConfirm();
              setOpen(false);
            }}
          >
            削除する
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
