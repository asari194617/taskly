import * as React from "react";
import {
  type DeadlineMeta,
  sinceUntilFromMeta,
} from "@/components/DeadlinePicker";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import {
  createTask as createTaskRequest,
  deleteTask as deleteTaskRequest,
  reportTask,
  updateTask as updateTaskRequest,
  uploadTaskImage,
  type Task,
} from "./api";
import { loadCompletionDefaults } from "./completion-defaults";
import { runOptimisticMutation } from "./state.mjs";
import type { TaskFormImage } from "./TaskFormFields";

export function useTaskActions(
  items: Task[] | null,
  setItems: React.Dispatch<React.SetStateAction<Task[] | null>>,
) {
  const { dict } = useLang();
  const [completionDefaults, setCompletionDefaults] = React.useState<Record<string, boolean>>({ QUIZ: true });
  const [newCompletionDisabled, setNewCompletionDisabled] = React.useState<boolean | undefined>();
  const [editCompletionDisabled, setEditCompletionDisabled] = React.useState(false);
  React.useEffect(() => {
    let active = true;
    loadCompletionDefaults().then((rows) => {
      if (active) setCompletionDefaults(Object.fromEntries(rows.map((row) => [row.type, row.is_completion_disabled])));
    }).catch((error) => {
      if (active) toast({ variant: "destructive", title: dict.tasks.loadFailed, description: error?.message });
    });
    return () => { active = false; };
  }, []);
  const createInFlight = React.useRef(false);
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
  const [newImages, setNewImages] = React.useState<TaskFormImage[]>([]);
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
  const [editImages, setEditImages] = React.useState<TaskFormImage[]>([]);
  async function createTask() {
    if (createInFlight.current) return false;
    if (!newTitle.trim() || !newSubjectId.trim() || !newDate.trim()) {
      toast({
        variant: "destructive",
        title: dict.toast.requiredFields,
      });
      return false;
    }
    createInFlight.current = true;
    setCreating(true);
    try {
      const imageKeys = await Promise.all(
        newImages.flatMap((image) => image.file ? [uploadTaskImage(image.file)] : []),
      );
      const created = await createTaskRequest({
        title: newTitle.trim(),
        subject_id: parseInt(newSubjectId, 10),
        is: newRequire,
        scr: newSelfCheck,
        completionDisabled: newCompletionDisabled,
        deadline: newDate,
        ...sinceUntilFromMeta(newDate, newMeta), // enums
        description: newDesc || undefined,
        type: newType || undefined,
        image_keys: imageKeys,
      });
      setItems((prev) => (prev ? [created, ...prev] : [created]));
      setNewTitle("");
      setNewSubjectId("");
      setNewDate("");
      setNewMeta(null);
      setNewRequire(false);
      setNewSelfCheck(false);
      setNewCompletionDisabled(undefined);
      setNewType("");
      setNewDesc("");
      setNewImages([]);
      setCreateOpen(false);
      toast({ title: dict.toast.created });
      return true;
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.createFailed,
        description: e?.message,
      });
      return false;
    } finally {
      createInFlight.current = false;
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
    setEditCompletionDisabled(!!t.is_completion_disabled);
    setEditType(t.type || "");
    setEditDesc(t.description || "");
    const keys = t.image_keys ?? [];
    const urls = t.image_urls ?? (t.image_url ? [t.image_url] : []);
    setEditImages(keys.map((key, index) => ({
      id: key,
      key,
      url: urls[index],
    })));
  }
  function cancelEdit() {
    setEditingId(null);
    setEditTitle("");
    setEditSubjectId("");
    setEditDate("");
    setEditMeta(null);
    setEditRequire(false);
    setEditSelfCheck(false);
    setEditType("");
    setEditDesc("");
    setEditImages([]);
  }
  async function saveEdit(id: number) {
    try {
      const uploadedKeys = await Promise.all(
        editImages.flatMap((image) => image.file ? [uploadTaskImage(image.file)] : []),
      );
      const imageKeys = [
        ...editImages.flatMap((image) => image.key ? [image.key] : []),
        ...uploadedKeys,
      ];
      const updated = await updateTaskRequest(id, {
        title: editTitle,
        subject_id: editSubjectId ? parseInt(editSubjectId, 10) : undefined,
        is: editRequire,
        scr: editSelfCheck,
        completionDisabled: editCompletionDisabled,
        deadline: editDate || undefined,
        ...sinceUntilFromMeta(editDate, editMeta),
        description: editDesc,
        type: editType,
        image_keys: imageKeys,
      });
      setItems((current) => current?.map((task) => task.id === id ? { ...task, ...updated, is_completed: updated.is_completion_disabled ? false : task.is_completed } : task) ?? null);
      toast({ title: dict.toast.updated });
      cancelEdit();
    } catch (e: any) {
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
    try {
      await runOptimisticMutation(
        prev,
        prev.filter((x) => x.id !== id),
        setItems,
        () => deleteTaskRequest(id),
      );
      toast({ title: dict.toast.deleted });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.deleteFailed,
        description: e?.message,
      });
    }
  }

  // Delete confirmation dialog state
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteTargetId, setDeleteTargetId] = React.useState<number | null>(
    null,
  );
  // Report dialog state
  const [reportOpen, setReportOpen] = React.useState(false);
  const [reportTargetId, setReportTargetId] = React.useState<number | null>(
    null,
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

  const createValues = {
    title: newTitle,
    subjectId: newSubjectId,
    date: newDate,
    meta: newMeta,
    requireHandIn: newRequire,
    selfCheck: newSelfCheck,
    completionDisabled: newCompletionDisabled ?? completionDefaults[newType || "OTHER"] ?? false,
    type: newType,
    description: newDesc,
    images: newImages,
  };
  const createSetters = {
    title: setNewTitle,
    subjectId: setNewSubjectId,
    date: setNewDate,
    meta: setNewMeta,
    requireHandIn: setNewRequire,
    selfCheck: setNewSelfCheck,
    completionDisabled: setNewCompletionDisabled,
    type: (value: string) => { setNewType(value); setNewCompletionDisabled(undefined); },
    description: setNewDesc,
    images: setNewImages,
  };
  const editValues = {
    title: editTitle,
    subjectId: editSubjectId,
    date: editDate,
    meta: editMeta,
    requireHandIn: editRequire,
    selfCheck: editSelfCheck,
    completionDisabled: editCompletionDisabled,
    type: editType,
    description: editDesc,
    images: editImages,
  };
  const editSetters = {
    title: setEditTitle,
    subjectId: setEditSubjectId,
    date: setEditDate,
    meta: setEditMeta,
    requireHandIn: setEditRequire,
    selfCheck: setEditSelfCheck,
    completionDisabled: setEditCompletionDisabled,
    type: setEditType,
    description: setEditDesc,
    images: setEditImages,
  };

  return {
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
  };
}
