import * as React from "react";
import { Button } from "@/components/ui/button";
import { TaskFormFields } from "@/features/tasks/TaskFormFields";
import { useTaskActions } from "@/features/tasks/useTaskActions";
import { loadSubjects, type Subject, type Task } from "@/features/tasks/api";
import { useLang } from "@/react/LanguageProvider";

export default function TaskCreatePage() {
  const { lang, dict } = useLang();
  const [items, setItems] = React.useState<Task[] | null>(null);
  const { createValues, createSetters, createTask, creating } = useTaskActions(
    items,
    setItems,
  );
  const [subjects, setSubjects] = React.useState<Subject[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const load = React.useCallback(() => {
    setLoading(true);
    setError("");
    loadSubjects()
      .then(setSubjects)
      .catch((e) => setError(e?.message || dict.tasks.loadFailed))
      .finally(() => setLoading(false));
  }, [dict.tasks.loadFailed]);
  React.useEffect(load, [load]);
  const ready =
    !loading &&
    !error &&
    !!createValues.title.trim() &&
    !!createValues.subjectId &&
    !!createValues.date;
  return (
    <div className="mx-auto max-w-xl space-y-6 pb-[env(safe-area-inset-bottom)]">
      <a
        href="/home"
        className="inline-flex min-h-11 items-center text-sm underline underline-offset-4"
      >
        {lang === "ja" ? "← 課題一覧に戻る" : "← Back to tasks"}
      </a>
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold">{dict.tasks.createTaskTitle}</h1>
        <p className="text-sm text-muted-foreground">
          {lang === "ja"
            ? "タイトル・教科・期限だけで登録できます。詳細は後から編集できます。"
            : "Start with a title, subject and deadline. You can edit the details later."}
        </p>
      </div>
      {error && (
        <div role="alert" className="space-y-2">
          <p>{error}</p>
          <Button type="button" onClick={load}>
            {dict.common.reload}
          </Button>
        </div>
      )}
      <form
        className="space-y-6"
        onSubmit={async (event) => {
          event.preventDefault();
          if (ready && !creating && (await createTask()))
            window.location.assign("/home");
        }}
      >
        <fieldset disabled={creating || loading} className="min-w-0">
          <TaskFormFields
            id="nt-page"
            values={createValues}
            set={createSetters}
            subjects={subjects}
            compact
          />
        </fieldset>
        <Button
          type="submit"
          disabled={!ready || creating}
          className="h-12 w-full text-base"
        >
          {creating
            ? lang === "ja"
              ? "登録中…"
              : "Creating…"
            : dict.tasks.create}
        </Button>
      </form>
    </div>
  );
}
