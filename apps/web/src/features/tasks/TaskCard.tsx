import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { MoreVertical, Pencil, Trash2, Save, X } from "lucide-react";
import {
  TaskFormFields,
  type TaskFormValues,
  type TaskFormSetters,
} from "./TaskFormFields";
import type { Subject, Task } from "./api";
import { deriveLabelFromEnums } from "@/components/DeadlinePicker";
import { useLang } from "@/react/LanguageProvider";
import { t } from "@/lib/i18n";

function localizeTaskType(
  type: string | null | undefined,
  dict: ReturnType<typeof t>["tasks"],
): string {
  if (!type) return "";
  switch (type.toUpperCase()) {
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

export function TaskCard({
  t,
  pendingInfo,
  isEditing,
  canManage,
  subjects,
  editValues,
  editSetters,
  toggleWithPending,
  saveEdit,
  cancelEdit,
  beginEdit,
  onDelete,
  onReport,
}: {
  t: Task;
  pendingInfo: boolean;
  isEditing: boolean;
  canManage: boolean;
  subjects: Subject[];
  editValues: TaskFormValues;
  editSetters: TaskFormSetters;
  toggleWithPending: (task: Task) => void;
  saveEdit: (id: number) => void;
  cancelEdit: () => void;
  beginEdit: (task: Task) => void;
  onDelete: (id: number) => void;
  onReport: (id: number) => void;
}) {
  const { lang, dict } = useLang();
  const deadlineLabel = t.deadline
    ? deriveLabelFromEnums(
        t.deadline,
        t.deadline_since,
        t.deadline_until,
        lang,
      )
    : "";
  return (
    <li
      key={t.id}
      onClick={(e) => {
        if (isEditing) return;
        if (
          (e.target as HTMLElement).closest(
            "button, input, textarea, label, select, a, [role='button'], [role='checkbox']",
          )
        ) return;
        toggleWithPending(t);
      }}
      className={
        "relative rounded-lg border p-4 pl-12 bg-card text-card-foreground transition-opacity " +
        (isEditing ? "" : "cursor-pointer ") +
        (pendingInfo ? " opacity-50" : "")
      }
    >
      <Checkbox
        checked={!!t.is_completed}
        disabled={isEditing}
        onCheckedChange={(checked) => {
          if (checked !== !!t.is_completed) {
            toggleWithPending(t);
          }
        }}
        aria-label={`${t.title}: ${dict.tasks.completion}`}
        className="absolute left-4 top-1/2 -translate-y-1/2"
      />
      {isEditing ? (
        <div className="space-y-3">
          <TaskFormFields
            id={`ed-${t.id}`}
            mode="edit"
            values={editValues}
            set={editSetters}
            subjects={subjects}
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={() => saveEdit(t.id)}>
              <Save className="h-4 w-4 mr-1" /> {dict.tasks.save}
            </Button>
            <Button size="sm" variant="secondary" onClick={cancelEdit}>
              <X className="h-4 w-4 mr-1" /> {dict.tasks.cancel}
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
                <Badge variant="destructive">{dict.tasks.requireHandIn}</Badge>
              )}
              {t.is_self_check_required && (
                <Badge>{dict.tasks.selfCheckBadge}</Badge>
              )}
              {canManage && t.is_hidden && (
                <Badge variant="outline">非表示</Badge>
              )}
            </div>
            <div className="font-medium">{t.title}</div>
            {(() => {
              const subj = subjects.find((s) => s.id === t.subject_id);
              const subjectTitle = subj
                ? lang === "en" && subj.name_en
                  ? subj.name_en
                  : subj.name_ja
                : `${dict.tasks.subjectId}: ${t.subject_id}`;
              return (
                <div className="text-sm text-muted-foreground">
                  {subjectTitle}
                  {deadlineLabel && ` ${deadlineLabel}`}
                </div>
              );
            })()}
            {t.description && (
              <p
                className="mt-1 whitespace-pre-line text-sm text-muted-foreground"
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
                    <Pencil className="h-4 w-4 mr-1" /> {dict.tasks.edit}
                  </Button>
                  {canManage && (
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => {
                        onDelete(t.id);
                      }}
                    >
                      <Trash2 className="h-4 w-4 mr-1" /> {dict.tasks.delete}
                    </Button>
                  )}
                  <div className="pt-2 border-t">
                    <Button
                      className="mt-2"
                      size="sm"
                      onClick={() => {
                        onReport(t.id);
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
}
