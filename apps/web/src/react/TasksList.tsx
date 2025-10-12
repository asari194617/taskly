import * as React from "react";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "../components/ui/empty";
import { Button } from "../components/ui/button";
import { gql } from "@/lib/graphql";
import { AlertCircle, Hourglass, FileText } from "lucide-react";

type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline?: string | null;
};

export default function TasksList() {
  const [items, setItems] = React.useState<Task[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  React.useEffect(() => {
    const token = localStorage.getItem("api_token");
    if (!token) {
      window.location.replace("/login");
      return;
    }
    gql<{ tasks: { items: Task[] } }>(
      `query{ tasks(limit:20){ items{ id title subject_id deadline } } }`
    )
      .then((d) => setItems(d.tasks.items))
      .catch((e) => setError(e?.message || "読み込みに失敗しました"));
  }, []);

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
          <Button onClick={() => (window.location.href = "/settings")}>
            設定へ
          </Button>
        </EmptyContent>
      </Empty>
    );
  }

  return (
    <ul className="space-y-3">
      {items.map((t) => {
        const deadline = t.deadline
          ? new Date(t.deadline).toLocaleString()
          : "期限なし";
        return (
          <li
            key={t.id}
            className="rounded-lg border p-4 bg-card text-card-foreground"
          >
            <div className="font-medium">{t.title}</div>
            <div className="text-sm text-muted-foreground">
              教科ID: {t.subject_id} / 期限: {deadline}
            </div>
          </li>
        );
      })}
    </ul>
  );
}
