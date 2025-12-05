import * as React from "react";
import { gql } from "@/lib/graphql";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";

type Report = {
  id: number;
  reporter_user_id?: number | null;
  task_id: number;
  reason: string;
  comment?: string | null;
  created_at: string;
};

export default function AdminReports() {
  const [idParam, setIdParam] = React.useState<number | null>(null);
  const [qTaskId, setQTaskId] = React.useState<string>("");
  const [items, setItems] = React.useState<Report[] | null>(null);
  const [one, setOne] = React.useState<Report | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    const usp = new URLSearchParams(window.location.search);
    const idStr = usp.get("id");
    setIdParam(idStr ? parseInt(idStr, 10) : null);
  }, []);

  React.useEffect(() => {
    (async () => {
      try {
        if (idParam != null) {
          const res = await gql<{ adminUserReport: Report }>(
            `query($id:Int!){ adminUserReport(id:$id){ id reporter_user_id task_id reason comment created_at } }`,
            { id: idParam }
          );
          setOne(res.adminUserReport || null);
          setItems(null);
        } else {
          const vars: any = {};
          if (qTaskId.trim()) vars.task_id = parseInt(qTaskId, 10) || undefined;
          const res = await gql<{ adminUserReports: Report[] }>(
            `query($task_id:Int){ adminUserReports(limit:50, task_id:$task_id){ id reporter_user_id task_id reason comment created_at } }`,
            vars
          );
          setItems(res.adminUserReports || []);
          setOne(null);
        }
        setError(null);
      } catch (e: any) {
        setError(e?.message || "Failed to load reports");
      }
    })();
  }, [idParam, qTaskId]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">User Reports</h1>
        <div className="flex items-center gap-2">
          <Input
            placeholder="Filter by task_id"
            value={qTaskId}
            onChange={(e) => setQTaskId(e.target.value)}
          />
          <Button onClick={() => setQTaskId("")}>Clear</Button>
        </div>
      </div>
      {error && (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              {/* you can add an icon here */}
            </EmptyMedia>
            <EmptyTitle>{error}</EmptyTitle>
          </EmptyHeader>
        </Empty>
      )}
      {one ? (
        <div className="border rounded-lg p-4">
          <div className="text-sm text-muted-foreground">ID: {one.id}</div>
          <div className="mt-1">Task ID: {one.task_id}</div>
          <div className="mt-1">
            Reporter User ID: {one.reporter_user_id ?? "(anonymous)"}
          </div>
          <div className="mt-1">Reason: {one.reason}</div>
          {one.comment && <div className="mt-1">Comment: {one.comment}</div>}
          <div className="mt-1 text-sm text-muted-foreground">
            Created: {new Date(one.created_at).toLocaleString()}
          </div>
          <div className="mt-3">
            <Button
              variant="secondary"
              onClick={() => {
                const url = new URL(window.location.href);
                url.searchParams.delete("id");
                window.location.href = url.toString();
              }}
            >
              Back to list
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {items && items.length === 0 && (
            <Empty className="border">
              <EmptyHeader>
                <EmptyMedia variant="icon" />
                <EmptyTitle>No reports</EmptyTitle>
              </EmptyHeader>
            </Empty>
          )}
          {items &&
            items.map((r) => (
              <div
                key={r.id}
                className="border rounded-lg p-4 flex items-start justify-between gap-3"
              >
                <div>
                  <div className="text-sm text-muted-foreground">
                    ID: {r.id}
                  </div>
                  <div>Task ID: {r.task_id}</div>
                  <div>Reporter: {r.reporter_user_id ?? "(anonymous)"}</div>
                  <div>Reason: {r.reason}</div>
                  {r.comment && <div>Comment: {r.comment}</div>}
                  <div className="text-sm text-muted-foreground">
                    {new Date(r.created_at).toLocaleString()}
                  </div>
                </div>
                <div>
                  <Button
                    onClick={() => {
                      const url = new URL(window.location.href);
                      url.searchParams.set("id", String(r.id));
                      window.location.href = url.toString();
                    }}
                  >
                    Open
                  </Button>
                </div>
              </div>
            ))}
        </div>
      )}
    </div>
  );
}
