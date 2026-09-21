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
  type Task,
} from "./api";
import { runOptimisticMutation } from "./state.mjs";

export function useTaskActions(
  items: Task[] | null,
  setItems: React.Dispatch<React.SetStateAction<Task[] | null>>,
) {
  const { dict } = useLang();
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
    const optimistic = prev.map((x) =>
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
        : x,
    );
    try {
      await runOptimisticMutation(prev, optimistic, setItems, () =>
        updateTaskRequest(id, {
          title: editTitle,
          subject_id: editSubjectId ? parseInt(editSubjectId, 10) : undefined,
          is: editRequire,
          scr: editSelfCheck,
          deadline: editDate || undefined,
          ...sinceUntilFromMeta(editDate, editMeta),
          description: editDesc,
          type: editType,
        }),
      );
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
    type: newType,
    description: newDesc,
  };
  const createSetters = {
    title: setNewTitle,
    subjectId: setNewSubjectId,
    date: setNewDate,
    meta: setNewMeta,
    requireHandIn: setNewRequire,
    selfCheck: setNewSelfCheck,
    type: setNewType,
    description: setNewDesc,
  };
  const editValues = {
    title: editTitle,
    subjectId: editSubjectId,
    date: editDate,
    meta: editMeta,
    requireHandIn: editRequire,
    selfCheck: editSelfCheck,
    type: editType,
    description: editDesc,
  };
  const editSetters = {
    title: setEditTitle,
    subjectId: setEditSubjectId,
    date: setEditDate,
    meta: setEditMeta,
    requireHandIn: setEditRequire,
    selfCheck: setEditSelfCheck,
    type: setEditType,
    description: setEditDesc,
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
