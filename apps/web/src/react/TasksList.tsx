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
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import {
  AlertCircle,
  Hourglass,
  FileText,
  Pencil,
  Trash2,
  Save,
  X,
} from "lucide-react";

type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline?: string | null;
  description?: string | null;
  is_require_hand_in?: boolean;
};
type Subject = { id: number; title: string };

export default function TasksList() {
  const [items, setItems] = React.useState<Task[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [newTitle, setNewTitle] = React.useState("");
  const [newSubjectId, setNewSubjectId] = React.useState("");
  const [newDeadline, setNewDeadline] = React.useState(""); // yyyy-MM-ddThh:mm
  const [newRequire, setNewRequire] = React.useState(false);
  const [editingId, setEditingId] = React.useState<number | null>(null);
  const [editTitle, setEditTitle] = React.useState("");
  const [editSubjectId, setEditSubjectId] = React.useState("");
  const [editDeadline, setEditDeadline] = React.useState("");
  const [editRequire, setEditRequire] = React.useState(false);
  const [subjects, setSubjects] = React.useState<Subject[]>([]);
  React.useEffect(() => {
    const token = localStorage.getItem("api_token");
    if (!token) {
      window.location.replace("/login");
      return;
    }
    Promise.all([
      gql<{ tasks: { items: Task[] } }>(
        `query{ tasks(limit:20){ items{ id title subject_id deadline } } }`
      ),
      gql<{ subjects: Subject[] }>(`query{ subjects{ id title } }`),
    ])
      .then(([t, s]) => {
        setItems(t.tasks.items);
        setSubjects(s.subjects || []);
      })
      .catch((e) => setError(e?.message || "読み込みに失敗しました"));
  }, []);

  function toDatetimeLocalValue(rfc3339?: string | null): string {
    if (!rfc3339) return "";
    const d = new Date(rfc3339);
    const pad = (n: number) => String(n).padStart(2, "0");
    const yyyy = d.getFullYear();
    const MM = pad(d.getMonth() + 1);
    const dd = pad(d.getDate());
    const hh = pad(d.getHours());
    const mm = pad(d.getMinutes());
    return `${yyyy}-${MM}-${dd}T${hh}:${mm}`;
  }
  function toRFC3339FromLocal(local: string): string | undefined {
    if (!local || local.trim() === "") return undefined;
    // Interpret as local time, convert to UTC ISO
    const d = new Date(local);
    return d.toISOString();
  }

  async function createTask() {
    if (!newTitle.trim() || !newSubjectId.trim()) {
      toast({
        variant: "destructive",
        title: "タイトルとサブジェクトは必須です",
      });
      return;
    }
    setCreating(true);
    try {
      const res = await gql<{ createTask: Task }>(
        `mutation($title:String!,$subject_id:Int!,$is:Boolean,$deadline:String){
          createTask(title:$title, subject_id:$subject_id, is_require_hand_in:$is, deadline:$deadline){ id title subject_id deadline }
        }`,
        {
          title: newTitle.trim(),
          subject_id: parseInt(newSubjectId, 10),
          is: newRequire,
          deadline: toRFC3339FromLocal(newDeadline),
        }
      );
      setItems((prev) => (prev ? [res.createTask, ...prev] : [res.createTask]));
      setNewTitle("");
      setNewSubjectId("");
      setNewDeadline("");
      setNewRequire(false);
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
    setEditDeadline(toDatetimeLocalValue(t.deadline));
    setEditRequire(!!t.is_require_hand_in);
  }
  function cancelEdit() {
    setEditingId(null);
    setEditTitle("");
    setEditSubjectId("");
    setEditDeadline("");
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
              deadline: toRFC3339FromLocal(editDeadline) || null,
              is_require_hand_in: editRequire,
            }
          : x
      )
    );
    try {
      await gql<{ updateTask: Task }>(
        `mutation($id:Int!,$title:String,$subject_id:Int,$is:Boolean,$deadline:String){
          updateTask(id:$id, title:$title, subject_id:$subject_id, is_require_hand_in:$is, deadline:$deadline){ id }
        }`,
        {
          id,
          title: editTitle,
          subject_id: editSubjectId ? parseInt(editSubjectId, 10) : undefined,
          is: editRequire,
          deadline: toRFC3339FromLocal(editDeadline),
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
                <select
                  id="nt-subject"
                  className="w-full h-9 rounded-md border border-input bg-background px-3 py-1 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                  value={newSubjectId}
                  onChange={(e) => setNewSubjectId(e.target.value)}
                >
                  <option value="">選択してください</option>
                  {subjects.map((s) => (
                    <option key={s.id} value={String(s.id)}>
                      {s.title}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <Label htmlFor="nt-deadline">期限（任意）</Label>
                <Input
                  id="nt-deadline"
                  type="datetime-local"
                  value={newDeadline}
                  onChange={(e) => setNewDeadline(e.target.value)}
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
      {/* New Task Form */}
      <div className="rounded-lg border p-4">
        <div className="text-sm font-medium mb-3">新規作成</div>
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
            <Label htmlFor="nt-subject-2">サブジェクトID</Label>
            <Input
              id="nt-subject-2"
              value={newSubjectId}
              onChange={(e) => setNewSubjectId(e.target.value)}
              placeholder="例: 1"
            />
          </div>
          <div>
            <Label htmlFor="nt-deadline-2">期限（任意）</Label>
            <Input
              id="nt-deadline-2"
              type="datetime-local"
              value={newDeadline}
              onChange={(e) => setNewDeadline(e.target.value)}
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
        </div>
        <div className="mt-3">
          <Button onClick={createTask} disabled={creating}>
            新規作成
          </Button>
        </div>
      </div>

      {/* List */}
      <ul className="space-y-3">
        {items.map((t) => {
          const deadline = t.deadline
            ? new Date(t.deadline).toLocaleString()
            : "期限なし";
          const isEditing = editingId === t.id;
          return (
            <li
              key={t.id}
              className="rounded-lg border p-4 bg-card text-card-foreground"
            >
              {isEditing ? (
                <div className="space-y-3">
                  <div>
                    <Label htmlFor={`ed-title-${t.id}`}>タイトル</Label>
                    <Input
                      id={`ed-title-${t.id}`}
                      value={editTitle}
                      onChange={(e) => setEditTitle(e.target.value)}
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <Label htmlFor={`ed-subject-${t.id}`}>サブジェクト</Label>
                      <select
                        id={`ed-subject-${t.id}`}
                        className="w-full h-9 rounded-md border border-input bg-background px-3 py-1 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                        value={editSubjectId}
                        onChange={(e) => setEditSubjectId(e.target.value)}
                      >
                        <option value="">選択してください</option>
                        {subjects.map((s) => (
                          <option key={s.id} value={String(s.id)}>
                            {s.title}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <Label htmlFor={`ed-deadline-${t.id}`}>
                        期限（任意）
                      </Label>
                      <Input
                        id={`ed-deadline-${t.id}`}
                        type="datetime-local"
                        value={editDeadline}
                        onChange={(e) => setEditDeadline(e.target.value)}
                      />
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id={`ed-req-${t.id}`}
                      checked={editRequire}
                      onCheckedChange={setEditRequire}
                    />
                    <Label htmlFor={`ed-req-${t.id}`}>提出必須</Label>
                  </div>
                  <div className="flex gap-2">
                    <Button size="sm" onClick={() => saveEdit(t.id)}>
                      <Save className="h-4 w-4 mr-1" /> 保存
                    </Button>
                    <Button size="sm" variant="secondary" onClick={cancelEdit}>
                      <X className="h-4 w-4 mr-1" /> キャンセル
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="font-medium">{t.title}</div>
                    <div className="text-sm text-muted-foreground">
                      教科ID: {t.subject_id} / 期限: {deadline}
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => beginEdit(t)}
                    >
                      <Pencil className="h-4 w-4 mr-1" /> 編集
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => deleteTask(t.id)}
                    >
                      <Trash2 className="h-4 w-4 mr-1" /> 削除
                    </Button>
                  </div>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
