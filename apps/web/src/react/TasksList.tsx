import * as React from "react";
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import {
  taskRows,
  visibleTasks,
  type CompletionFilter,
} from "@/features/tasks/list-model";
import { useTaskSearch } from "@/features/tasks/useTaskSearch";
import { useTaskCompletion } from "@/features/tasks/useTaskCompletion";
import { useTaskActions } from "@/features/tasks/useTaskActions";
import { TaskFormFields } from "@/features/tasks/TaskFormFields";
import { TaskCard } from "@/features/tasks/TaskCard";
import {
  TaskReportDialog,
  TasksListDeleteDialog,
} from "@/features/tasks/TaskDialogs";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
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
import { toast } from "@/hooks/use-toast";
import { AlertCircle, FileText, Plus, Search } from "lucide-react";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import { useLang } from "@/react/LanguageProvider";
import {
  loadSubjects,
  loadViewerRole,
  type Subject,
} from "@/features/tasks/api";

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

export default function TasksList() {
  const { dict } = useLang();

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
    [dict.tasks],
  );

  const {
    items,
    setItems,
    nextCursor,
    loadingMore,
    loadMoreError,
    error,
    setError,
    q,
    qInput,
    setQInput,
    submitSearch,
    loadMore,
  } = useTaskSearch(dict.tasks.loadFailed);
  const [isAdmin, setIsAdmin] = React.useState(false);
  const [isManager, setIsManager] = React.useState(false);
  const [completedFilter, setCompletedFilter] =
    React.useState<CompletionFilter>("all");
  const { pending, toggleWithPending } = useTaskCompletion(
    setItems,
    (error: any) => {
      toast({
        variant: "destructive",
        title: dict.toast.updateFailed,
        description: error?.message,
      });
    },
  );
  const {
    creating,
    createOpen,
    setCreateOpen,
    editingId,
    editTitle,
    editSubjectId,
    editDate,
    editMeta,
    editRequire,
    editSelfCheck,
    editType,
    editDesc,
    createValues,
    createSetters,
    editValues,
    editSetters,
    createTask,
    beginEdit,
    cancelEdit,
    saveEdit,
    deleteTask,
    deleteOpen,
    setDeleteOpen,
    deleteTargetId,
    setDeleteTargetId,
    reportOpen,
    setReportOpen,
    reportTargetId,
    setReportTargetId,
    reportReason,
    setReportReason,
    reportComment,
    setReportComment,
    reportSubmittingId,
    submitReport,
  } = useTaskActions(items, setItems);
  const [subjects, setSubjects] = React.useState<Subject[]>([]);
  const isDesktop = useMediaQuery("(min-width: 768px)");
  // One-time bootstrap: subjects and my role; avoid reloading on each search
  React.useEffect(() => {
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

  const shownItems = React.useMemo(
    () => visibleTasks(items ?? [], q, completedFilter),
    [items, q, completedFilter],
  );
  const rows = React.useMemo(
    () => taskRows(shownItems, formatHeadingForDate, dict.tasks.noDeadline),
    [shownItems, formatHeadingForDate, dict.tasks.noDeadline],
  );

  const listRef = React.useRef<HTMLDivElement>(null);
  const [scrollMargin, setScrollMargin] = React.useState(0);
  React.useLayoutEffect(() => {
    const update = () => {
      if (listRef.current) {
        const top =
          listRef.current.getBoundingClientRect().top + window.scrollY;
        setScrollMargin((current) => (current === top ? current : top));
      }
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  });
  const virtualizer = useWindowVirtualizer({
    count: rows.length + (nextCursor ? 1 : 0),
    estimateSize: (index) =>
      index === rows.length ? 64 : rows[index]?.heading ? 158 : 122,
    getItemKey: (index) => rows[index]?.task.id ?? "load-more",
    overscan: 5,
    scrollMargin,
    useFlushSync: false,
  });
  const virtualItems = virtualizer.getVirtualItems();
  React.useEffect(() => {
    if (
      nextCursor &&
      !loadingMore &&
      !loadMoreError &&
      virtualItems.some((item) => item.index === rows.length)
    )
      void loadMore();
  }, [
    nextCursor,
    loadingMore,
    loadMoreError,
    virtualItems,
    rows.length,
    loadMore,
  ]);

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
                  submitSearch();
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

      {/* Completion filters ("すべて" omitted). Activate the selected filter to clear it. */}
      <div className="flex flex-wrap gap-2 -mt-2">
        {(
          [
            { key: "incomplete", label: "未完了" },
            { key: "completed", label: "完了済み" },
          ] as const
        ).map((f) => {
          const active = completedFilter === f.key;
          return (
            <Badge
              key={f.key}
              asChild
              className={
                "cursor-pointer select-none px-3 py-1" +
                (active
                  ? " bg-primary text-primary-foreground"
                  : " bg-muted text-muted-foreground")
              }
            >
              <button
                type="button"
                aria-pressed={active}
                onClick={() =>
                  setCompletedFilter((prev) => (prev === f.key ? "all" : f.key))
                }
              >
                {f.label}
              </button>
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
          <TaskFormFields
            id="nt-dialog"
            values={createValues}
            set={createSetters}
            subjects={subjects}
          />
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
          <div className="px-4 pb-4 space-y-3">
            <TaskFormFields
              id="nt-drawer"
              values={createValues}
              set={createSetters}
              subjects={subjects}
            />
            <Button onClick={createTask} disabled={creating} className="w-full">
              {dict.tasks.create}
            </Button>
          </div>
        </DrawerContent>
      </Drawer>
      {/* Empty states below toolbar */}
      {shownItems.length === 0 &&
        !nextCursor &&
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
                <TaskFormFields
                  id="nt-inline"
                  values={createValues}
                  set={createSetters}
                  subjects={subjects}
                />
                <Button onClick={createTask} disabled={creating}>
                  {dict.tasks.create}
                </Button>
              </div>
            </EmptyContent>
          </Empty>
        ))}

      {/* Only visible task rows stay mounted while the window scrolls. */}
      <div
        ref={listRef}
        className="relative"
        style={{ height: virtualizer.getTotalSize() }}
      >
        {virtualItems.map((virtualRow) => {
          const row = rows[virtualRow.index];
          const t = row?.task;
          if (!t) {
            return (
              <div
                key="load-more"
                data-index={virtualRow.index}
                ref={virtualizer.measureElement}
                className="absolute left-0 top-0 w-full py-3 text-center"
                style={{
                  transform: `translateY(${virtualRow.start - scrollMargin}px)`,
                }}
              >
                {loadMoreError ? (
                  <Button variant="outline" onClick={() => void loadMore()}>
                    {dict.common.reload}
                  </Button>
                ) : (
                  <Skeleton className="mx-auto h-8 w-40" />
                )}
              </div>
            );
          }
          const isEditing = editingId === t.id;
          const pendingInfo = pending[t.id];
          return (
            <div
              key={t.id}
              data-index={virtualRow.index}
              ref={virtualizer.measureElement}
              className="absolute left-0 top-0 w-full pb-3"
              style={{
                transform: `translateY(${virtualRow.start - scrollMargin}px)`,
              }}
            >
              {row.heading && (
                <h3 className="text-sm font-medium text-muted-foreground mb-2">
                  {row.heading}
                </h3>
              )}
              <ul>
                <TaskCard
                  t={t}
                  pendingInfo={!!pendingInfo}
                  isEditing={isEditing}
                  canManage={isAdmin || isManager}
                  subjects={subjects}
                  editValues={editValues}
                  editSetters={editSetters}
                  toggleWithPending={toggleWithPending}
                  saveEdit={saveEdit}
                  cancelEdit={cancelEdit}
                  beginEdit={beginEdit}
                  onDelete={(id) => {
                    setDeleteTargetId(id);
                    setDeleteOpen(true);
                  }}
                  onReport={(id) => {
                    setReportTargetId(id);
                    setReportOpen(true);
                  }}
                />
              </ul>
            </div>
          );
        })}
      </div>
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
      <TaskReportDialog
        open={reportOpen}
        setOpen={setReportOpen}
        targetId={reportTargetId}
        reason={reportReason}
        setReason={setReportReason}
        comment={reportComment}
        setComment={setReportComment}
        submittingId={reportSubmittingId}
        onSubmit={submitReport}
      />
    </div>
  );
}
