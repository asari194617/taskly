"use client";
import * as React from "react";
import { useToast } from "@/hooks/use-toast";
import { getApiBase, webFetch } from "@/lib/api";

type FlaggedRow = {
  id: number;
  task_id: number;
  task_title: string;
  category: string;
  severity: string;
  task_hidden: boolean;
};

type Policy = {
  default_auto_hide: "low" | "medium" | "high";
  auto_hide_by_category?: Record<string, string>;
};

async function api<T = any>(path: string, opts: RequestInit = {}): Promise<T> {
  const r = await webFetch(`${getApiBase()}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  if (!r.ok) {
    const t = await r.text();
    throw new Error(t || r.statusText);
  }
  return r.json();
}

export default function Moderation() {
  const [items, setItems] = React.useState<FlaggedRow[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [policy, setPolicy] = React.useState<Policy>({
    default_auto_hide: "high",
    auto_hide_by_category: { threat: "medium" },
  });
  const { toast } = useToast();

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const [rows, pol] = await Promise.all([
        api<FlaggedRow[]>("/admin/moderation/flagged?limit=50"),
        api<Policy>("/admin/moderation/settings"),
      ]);
      setItems(rows || []);
      if (pol) setPolicy(pol);
    } catch (e) {
      console.error(e);
      toast({
        title: "Failed to load moderation data",
        variant: "destructive",
      });
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const act = async (id: number, action: "hide" | "unhide") => {
    try {
      await webFetch(`${getApiBase()}/admin/moderation/tasks/${id}/${action}`, {
        method: "POST",
      });
      await load();
    } catch (e) {
      toast({
        title: "Action failed",
        description: "Please try again.",
        variant: "destructive",
      });
    }
  };

  const savePolicy = async () => {
    try {
      await webFetch(`${getApiBase()}/admin/moderation/settings`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(policy),
      });
      toast({
        title: "Policy saved",
        description: "Moderation policy updated successfully.",
      });
    } catch (e) {
      toast({ title: "Failed to save policy", variant: "destructive" });
    }
  };

  return (
    <section className="p-6">
      <h1 className="text-xl font-semibold mb-4">Flagged contents</h1>
      <div className="mb-6 p-4 border rounded bg-slate-900 text-slate-200">
        <h2 className="font-medium mb-2">Policy</h2>
        <div className="flex gap-2 items-center mb-2">
          <label className="w-48">Default auto-hide:</label>
          <select
            value={policy.default_auto_hide}
            onChange={(e) =>
              setPolicy({ ...policy, default_auto_hide: e.target.value as any })
            }
          >
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
          </select>
        </div>
        <div className="mb-2">
          <label className="block mb-1">Per-category overrides (JSON)</label>
          <textarea
            className="w-full h-24"
            value={JSON.stringify(policy.auto_hide_by_category || {}, null, 2)}
            onChange={(e) => {
              try {
                const v = JSON.parse(e.target.value);
                setPolicy({ ...policy, auto_hide_by_category: v });
              } catch {}
            }}
          />
        </div>
        <button className="px-3 py-1 rounded bg-blue-600" onClick={savePolicy}>
          Save policy
        </button>
      </div>
      {loading ? (
        <div>Loading...</div>
      ) : (
        <table className="w-full text-left border-collapse">
          <thead>
            <tr>
              <th className="border-b p-2">ID</th>
              <th className="border-b p-2">Task</th>
              <th className="border-b p-2">Category</th>
              <th className="border-b p-2">Severity</th>
              <th className="border-b p-2">Hidden</th>
              <th className="border-b p-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((r) => (
              <tr key={r.id}>
                <td className="border-b p-2">{r.id}</td>
                <td className="border-b p-2">{r.task_title}</td>
                <td className="border-b p-2">{r.category}</td>
                <td className="border-b p-2">{r.severity}</td>
                <td className="border-b p-2">{String(r.task_hidden)}</td>
                <td className="border-b p-2">
                  <button
                    className="px-2 py-1 mr-2 bg-emerald-700 rounded"
                    onClick={() => act(r.task_id, "unhide")}
                  >
                    Unhide
                  </button>
                  <button
                    className="px-2 py-1 bg-rose-700 rounded"
                    onClick={() => act(r.task_id, "hide")}
                  >
                    Hide
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
