import React, { useEffect, useMemo, useState } from "react";
import { gql } from "../lib/graphql";
// shadcn/ui primitives
// Use existing minimal UI wrappers or plain elements styled via shadcn classes
const Button = (
  props: React.ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: "secondary" | "destructive";
  }
) => {
  const base = "px-3 py-1 rounded bg-primary text-primary-foreground";
  const secondary = "px-3 py-1 rounded bg-secondary text-secondary-foreground";
  const destructive =
    "px-3 py-1 rounded bg-destructive text-destructive-foreground";
  const cls =
    props.variant === "secondary"
      ? secondary
      : props.variant === "destructive"
      ? destructive
      : base;
  const { variant, className, ...rest } = props as any;
  return (
    <button className={(className ? className + " " : "") + cls} {...rest} />
  );
};
const Input = (props: React.InputHTMLAttributes<HTMLInputElement>) => (
  <input
    {...props}
    className={
      (props.className ? props.className + " " : "") +
      "px-2 py-1 rounded border"
    }
  />
);
const Card: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="border rounded p-3 space-y-2">{children}</div>
);
const Select = (props: React.SelectHTMLAttributes<HTMLSelectElement>) => (
  <select
    {...props}
    className={
      (props.className ? props.className + " " : "") +
      "px-2 py-1 rounded border"
    }
  />
);

type Cycle = {
  id: number;
  begin_date: string;
  finish_date: string;
  description: string;
};
type Group = {
  id: number;
  cycle_id: number;
  position: number;
  user_ids: number[];
};

export default function DayShiftAdmin() {
  const [indexVal, setIndexVal] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(true);
  const [cycles, setCycles] = useState<Cycle[]>([]);
  const [groupsByCycle, setGroupsByCycle] = useState<Record<number, Group[]>>(
    {}
  );

  const fetchAll = async () => {
    setLoading(true);
    try {
      const idxResp = await gql<{ dayShiftIndex: number }>(
        `query{ dayShiftIndex }`
      );
      setIndexVal(idxResp.dayShiftIndex ?? 0);
      const cycResp = await gql<{ adminDayshiftCycles: Cycle[] }>(
        `query{ adminDayshiftCycles{ id begin_date finish_date description } }`
      );
      setCycles(cycResp.adminDayshiftCycles || []);
      const groupsMap: Record<number, Group[]> = {};
      for (const c of cycResp.adminDayshiftCycles || []) {
        const grResp = await gql<{ adminDayshiftGroups: Group[] }>(
          `query($cid:Int!){ adminDayshiftGroups(cycle_id:$cid){ id cycle_id position user_ids } }`,
          { cid: c.id }
        );
        groupsMap[c.id] = grResp.adminDayshiftGroups || [];
      }
      setGroupsByCycle(groupsMap);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAll();
  }, []);

  const onSaveIndex = async () => {
    await gql(`mutation($v:Int!){ adminUpdateDayShiftIndex(index:$v) }`, {
      v: indexVal,
    });
    await fetchAll();
  };

  const [newBegin, setNewBegin] = useState("");
  const [newFinish, setNewFinish] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const createCycle = async () => {
    if (!newBegin || !newFinish || !newDesc) return;
    await gql(
      `mutation($bd:String!,$fd:String!,$desc:String!){ adminDayshiftCreateCycle(begin_date:$bd, finish_date:$fd, description:$desc){ id } }`,
      { bd: newBegin, fd: newFinish, desc: newDesc }
    );
    setNewBegin("");
    setNewFinish("");
    setNewDesc("");
    await fetchAll();
  };

  const deleteCycle = async (id: number) => {
    await gql(`mutation($id:Int!){ adminDayshiftDeleteCycle(id:$id) }`, { id });
    await fetchAll();
  };

  const [newGroupCycleId, setNewGroupCycleId] = useState<number>(0);
  const [newGroupMembers, setNewGroupMembers] = useState<string>("");
  const createGroup = async () => {
    if (!newGroupCycleId) return;
    const members = (newGroupMembers || "")
      .split(",")
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n));
    if (members.length === 0) return;
    await gql(
      `mutation($cid:Int!,$uids:[Int!]!){ adminDayshiftCreateGroup(cycle_id:$cid, user_ids:$uids){ id } }`,
      { cid: newGroupCycleId, uids: members }
    );
    setNewGroupMembers("");
    await fetchAll();
  };

  const deleteGroup = async (cycleId: number, groupId: number) => {
    await gql(
      `mutation($cid:Int!,$gid:Int!){ adminDayshiftDeleteGroup(cycle_id:$cid, group_id:$gid) }`,
      { cid: cycleId, gid: groupId }
    );
    await fetchAll();
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div className="space-y-6">
      <Card>
        <div className="space-y-2">
          <h3 className="text-base font-semibold">インデックス制御</h3>
          <div className="flex items-center gap-3">
            <Input
              type="number"
              value={String(indexVal)}
              onChange={(e) => setIndexVal(parseInt(e.target.value || "0", 10))}
              className="w-28"
            />
            <Button onClick={onSaveIndex}>更新</Button>
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-3">
          <h3 className="text-base font-semibold">サイクル</h3>
          <div className="space-y-2">
            {cycles.map((c) => (
              <div key={c.id} className="flex items-center gap-2">
                <span className="px-2 py-1 rounded border">
                  #{c.id} {c.description} ({c.begin_date} → {c.finish_date})
                </span>
                <Button variant="secondary" onClick={() => deleteCycle(c.id)}>
                  削除
                </Button>
              </div>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Input
              placeholder="開始日 YYYY-MM-DD"
              value={newBegin}
              onChange={(e) => setNewBegin(e.target.value)}
            />
            <Input
              placeholder="終了日 YYYY-MM-DD"
              value={newFinish}
              onChange={(e) => setNewFinish(e.target.value)}
            />
            <Input
              placeholder="説明"
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
              className="w-64"
            />
            <Button onClick={createCycle}>追加</Button>
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-3">
          <h3 className="text-base font-semibold">グループ</h3>
          {cycles.map((c) => (
            <div key={c.id} className="space-y-2">
              <div className="text-sm font-medium">
                Cycle #{c.id} {c.description}
              </div>
              {(groupsByCycle[c.id] || []).map((g) => (
                <div key={g.id} className="flex items-center gap-2">
                  <span className="px-2 py-1 rounded border">
                    Group #{g.id} pos:{g.position} members:{" "}
                    {g.user_ids.join(", ")}
                  </span>
                  <Button
                    variant="destructive"
                    onClick={() => deleteGroup(c.id, g.id)}
                  >
                    削除
                  </Button>
                </div>
              ))}
            </div>
          ))}
          <div className="flex items-center gap-2">
            <Select
              value={String(newGroupCycleId || "")}
              onChange={(e: any) =>
                setNewGroupCycleId(parseInt(e.target.value || "0", 10))
              }
            >
              <option value="">サイクル選択</option>
              {cycles.map((c) => (
                <option key={c.id} value={String(c.id)}>
                  #{c.id} {c.description}
                </option>
              ))}
            </Select>
            <Input
              placeholder="ユーザーID(カンマ区切り)"
              value={newGroupMembers}
              onChange={(e) => setNewGroupMembers(e.target.value)}
              className="w-64"
            />
            <Button onClick={createGroup}>追加</Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
