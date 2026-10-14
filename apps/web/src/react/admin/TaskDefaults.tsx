import * as React from "react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { toast } from "@/hooks/use-toast";
import { loadViewerRole } from "@/features/tasks/api";
import { useLang } from "@/react/LanguageProvider";
import {
  loadCompletionDefaults,
  saveCompletionDefault,
  taskTypeOptions,
} from "@/features/tasks/completion-defaults";

export default function TaskDefaults() {
  const { lang, dict } = useLang();
  const [values, setValues] = React.useState<Record<string, boolean> | null>(
    null,
  );
  const [canSave, setCanSave] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");
  React.useEffect(() => {
    Promise.all([loadCompletionDefaults(), loadViewerRole()])
      .then(([rows, role]) => {
        setValues(
          Object.fromEntries(
            rows.map((row) => [row.type, row.is_completion_disabled]),
          ),
        );
        setCanSave(role.isAdmin);
      })
      .catch((e) => setError(e.message));
  }, []);
  async function save() {
    if (!values || saving || !canSave) return;
    setSaving(true);
    try {
      // Each type is saved independently; failed saves leave the draft available for retry.
      for (const { type } of taskTypeOptions)
        await saveCompletionDefault(type, values[type]);
      toast({ title: dict.toast.updated });
      setError("");
    } catch (e: any) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  }
  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <h1 className="text-2xl font-semibold">
        {lang === "ja" ? "課題タイプの初期設定" : "Task type defaults"}
      </h1>
      <p className="text-sm text-muted-foreground">
        {lang === "ja"
          ? "完了マークを無効にするタイプを選択します。新規課題に適用され、課題ごとに変更できます。既存の課題は変更されません。"
          : "Choose which types disable completion marking for new tasks. Each task can override this. Existing tasks are unchanged."}
      </p>
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      {!values && !error && <p>{dict.common.loading}</p>}
      {values &&
        taskTypeOptions.map(({ type, label }) => (
          <div
            key={type}
            className="flex items-center justify-between rounded-lg border p-4"
          >
            <Label htmlFor={`default-${type}`}>
              {dict.tasks[label]} — {dict.tasks.disableCompletion}
            </Label>
            <Switch
              id={`default-${type}`}
              checked={values[type]}
              disabled={saving || !canSave}
              onCheckedChange={(value) =>
                setValues({ ...values, [type]: value })
              }
            />
          </div>
        ))}
      {canSave && (
        <Button onClick={save} disabled={!values || saving}>
          {dict.tasks.save}
        </Button>
      )}
    </div>
  );
}
