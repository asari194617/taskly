// removed initial simple implementation; keeping rich shadcn/ui version below
"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  CheckIcon,
  ChevronsUpDownIcon,
  XIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  PencilIcon,
  Calendar as CalendarIcon,
} from "lucide-react";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Calendar } from "@/components/ui/calendar";
import { Textarea } from "@/components/ui/textarea";

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
type AdminUser = {
  id: number;
  email: string;
  display_name: string;
};
type Assignment = {
  id: number;
  date: string; // YYYY-MM-DD
  cycle_id: number;
  status: string;
  user_ids: number[];
};

export default function AdminDayShift() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [cycles, setCycles] = React.useState<Cycle[]>([]);
  const [cid, setCid] = React.useState<string>("");
  const [autoCid, setAutoCid] = React.useState<string>("");
  const [groups, setGroups] = React.useState<Group[]>([]);
  const [begin, setBegin] = React.useState("");
  const [finish, setFinish] = React.useState("");
  const [desc, setDesc] = React.useState("");
  // cycle selection via combobox
  const [cycleOpen, setCycleOpen] = React.useState(false);
  const [cycleFilter, setCycleFilter] = React.useState("");
  // multi-user selection for new group via combobox
  const [userOpen, setUserOpen] = React.useState(false);
  const [userQuery, setUserQuery] = React.useState("");
  const [userOptions, setUserOptions] = React.useState<AdminUser[]>([]);
  const [selectedUsers, setSelectedUsers] = React.useState<AdminUser[]>([]);
  const [userMap, setUserMap] = React.useState<Record<number, AdminUser>>({});
  const [loadingCycles, setLoadingCycles] = React.useState(false);
  const [loadingGroups, setLoadingGroups] = React.useState(false);
  const [loadingUsers, setLoadingUsers] = React.useState(false);
  // assignments state
  const [assignments, setAssignments] = React.useState<Assignment[]>([]);
  const [assignStart, setAssignStart] = React.useState("");
  const [assignEnd, setAssignEnd] = React.useState("");
  const [loadingAssignments, setLoadingAssignments] = React.useState(false);
  // pagination state
  const [cyclePage, setCyclePage] = React.useState(1);
  const [groupPage, setGroupPage] = React.useState(1);
  const pageSize = 10;
  const [cycleTotal, setCycleTotal] = React.useState(0);
  const [groupTotal, setGroupTotal] = React.useState(0);
  // index config and prediction
  const [currentIndex, setCurrentIndex] = React.useState<number>(0);
  const [savingIndex, setSavingIndex] = React.useState(false);
  const [predictPage, setPredictPage] = React.useState(1);
  // current cycle id control
  const [currentCycleId, setCurrentCycleId] = React.useState<string>("");
  const [savingCycleId, setSavingCycleId] = React.useState(false);

  // edit members dialog state
  const [editOpen, setEditOpen] = React.useState(false);
  const [editGroup, setEditGroup] = React.useState<Group | null>(null);
  const [editSelected, setEditSelected] = React.useState<AdminUser[]>([]);
  // bulk add dialog state
  const [bulkOpen, setBulkOpen] = React.useState(false);
  const [bulkCSV, setBulkCSV] = React.useState("");
  const [bulkLoading, setBulkLoading] = React.useState(false);
  // edit assignment members dialog state
  const [editAsgOpen, setEditAsgOpen] = React.useState(false);
  const [editAsgDate, setEditAsgDate] = React.useState<string>("");
  const [editAsgSelected, setEditAsgSelected] = React.useState<AdminUser[]>([]);

  async function loadCycles() {
    try {
      setLoadingCycles(true);
      const d = await gql<{
        adminDayshiftCyclesPaginated: { items: Cycle[]; total: number };
        dayShiftCurrentCycle: number;
      }>(
        `query($page:Int,$size:Int){
          adminDayshiftCyclesPaginated(page:$page,page_size:$size){ items{ id begin_date finish_date description } total }
          dayShiftCurrentCycle
        }`,
        { page: cyclePage, size: pageSize }
      );
      setCycles(d.adminDayshiftCyclesPaginated?.items || []);
      setCycleTotal(d.adminDayshiftCyclesPaginated?.total || 0);
      const curId = d.dayShiftCurrentCycle;
      if (curId && !cid) setAutoCid(String(curId));
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoadingCycles(false);
    }
  }
  async function loadGroups() {
    const active = cid || autoCid;
    if (!active) {
      setGroups([]);
      return;
    }
    try {
      setLoadingGroups(true);
      const d = await gql<{
        adminDayshiftGroupsPaginated: { items: Group[]; total: number };
      }>(
        `query($cid:Int!,$page:Int,$size:Int){ adminDayshiftGroupsPaginated(cycle_id:$cid,page:$page,page_size:$size){ items{ id cycle_id position user_ids } total } }`,
        { cid: Number(active), page: groupPage, size: pageSize }
      );
      const arr = d.adminDayshiftGroupsPaginated?.items || [];
      setGroupTotal(d.adminDayshiftGroupsPaginated?.total || 0);
      setGroups(arr);
      // fetch user details for rendering
      const ids = Array.from(new Set(arr.flatMap((g) => g.user_ids)));
      if (ids.length > 0) {
        const ures = await gql<{ adminUsersByIds: AdminUser[] }>(
          `query($ids:[Int!]!){ adminUsersByIds(ids:$ids){ id email display_name } }`,
          { ids }
        );
        const um: Record<number, AdminUser> = {};
        for (const u of ures.adminUsersByIds || []) um[u.id] = u;
        setUserMap(um);
      } else {
        setUserMap({});
      }
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoadingGroups(false);
    }
  }
  async function loadAssignments() {
    try {
      setLoadingAssignments(true);
      // Require at least a cycle or start/end filter to avoid huge result sets
      const vars: any = {};
      if (cid) vars.cycle_id = Number(cid);
      if (assignStart) vars.start = assignStart;
      if (assignEnd) vars.end = assignEnd;
      const res = await gql<{ adminDayshiftAssignments: Assignment[] }>(
        `query($cycle_id:Int,$start:String,$end:String){ adminDayshiftAssignments(cycle_id:$cycle_id,start:$start,end:$end){ id date cycle_id status user_ids } }`,
        vars
      );
      const arr = res.adminDayshiftAssignments || [];
      setAssignments(arr);
      const ids = Array.from(new Set(arr.flatMap((a) => a.user_ids)));
      if (ids.length > 0) {
        const ures = await gql<{ adminUsersByIds: AdminUser[] }>(
          `query($ids:[Int!]!){ adminUsersByIds(ids:$ids){ id email display_name } }`,
          { ids }
        );
        setUserMap((prev) => {
          const cp = { ...prev } as Record<number, AdminUser>;
          for (const u of ures.adminUsersByIds || []) cp[u.id] = u;
          return cp;
        });
      }
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoadingAssignments(false);
    }
  }
  React.useEffect(() => {
    if (token) {
      loadCycles();
      gql<{ dayShiftIndex: number }>(`query{ dayShiftIndex }`)
        .then((d) => setCurrentIndex(d.dayShiftIndex ?? 0))
        .catch(() => setCurrentIndex(0));
      // Try new query; fallback to legacy adminDayshiftCurrentCycle
      gql<{ dayShiftCurrentCycle: number }>(`query{ dayShiftCurrentCycle }`)
        .then((d) => {
          const v = d.dayShiftCurrentCycle || 0;
          setCurrentCycleId(v ? String(v) : "");
        })
        .catch(async () => {
          try {
            const dd = await gql<{
              adminDayshiftCurrentCycle: { id: number } | null;
            }>(`query{ adminDayshiftCurrentCycle{ id } }`);
            const v = dd.adminDayshiftCurrentCycle?.id || 0;
            setCurrentCycleId(v ? String(v) : "");
          } catch {
            setCurrentCycleId("");
          }
        });
    }
  }, [token, cyclePage]);
  React.useEffect(() => {
    setGroupPage(1);
  }, [cid, autoCid]);
  React.useEffect(() => {
    const activeCid = cid || autoCid;
    if (!activeCid) {
      setGroups([]);
      setGroupTotal(0);
      return;
    }
    loadGroups();
  }, [cid, autoCid, groupPage]);
  // Auto load assignments when cycle or range changes (if any filter provided)
  React.useEffect(() => {
    if (cid || assignStart || assignEnd) {
      loadAssignments();
    } else {
      setAssignments([]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cid, assignStart, assignEnd]);

  async function createCycle() {
    try {
      await gql<{ adminDayshiftCreateCycle: { id: number } }>(
        `mutation($b:String!,$f:String!,$d:String!){ adminDayshiftCreateCycle(begin_date:$b, finish_date:$f, description:$d){ id } }`,
        { b: begin, f: finish, d: desc }
      );
      setBegin("");
      setFinish("");
      setDesc("");
      loadCycles();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }
  async function deleteCycle(id: number) {
    try {
      await gql<{ adminDayshiftDeleteCycle: boolean }>(
        `mutation($id:Int!){ adminDayshiftDeleteCycle(id:$id) }`,
        { id }
      );
      if (String(id) === cid) setCid("");
      loadCycles();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }
  async function createGroup() {
    if (!cid || selectedUsers.length === 0) return;
    try {
      await gql<{ adminDayshiftCreateGroup: { id: number } }>(
        `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftCreateGroup(cycle_id:$cid, user_ids:$ids){ id } }`,
        { cid: Number(cid), ids: selectedUsers.map((u) => u.id) }
      );
      setSelectedUsers([]);
      setUserQuery("");
      loadGroups();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }
  async function deleteGroup(id: number) {
    if (!cid) return;
    try {
      await gql<{ adminDayshiftDeleteGroup: boolean }>(
        `mutation($cid:Int!,$gid:Int!){ adminDayshiftDeleteGroup(cycle_id:$cid, group_id:$gid) }`,
        { cid: Number(cid), gid: id }
      );
      loadGroups();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }
  async function reorderGroups(newOrder: number[]) {
    try {
      await gql<{ adminDayshiftReorderGroups: boolean }>(
        `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftReorderGroups(cycle_id:$cid, group_ids:$ids) }`,
        { cid: Number(cid), ids: newOrder }
      );
      loadGroups();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "並び替えに失敗しました",
        description: e?.message,
      });
    }
  }
  async function moveUp(idx: number) {
    // We need full order for reordering stability; fetch ids for entire cycle
    const res = await gql<{ adminDayshiftGroupIds: number[] }>(
      `query($cid:Int!){ adminDayshiftGroupIds(cycle_id:$cid) }`,
      { cid: Number(cid) }
    );
    const order = res.adminDayshiftGroupIds || [];
    const globalIndex = (groupPage - 1) * pageSize + idx;
    if (globalIndex <= 0 || globalIndex >= order.length) return;
    [order[globalIndex - 1], order[globalIndex]] = [
      order[globalIndex],
      order[globalIndex - 1],
    ];
    reorderGroups(order);
  }
  async function moveDown(idx: number) {
    const res = await gql<{ adminDayshiftGroupIds: number[] }>(
      `query($cid:Int!){ adminDayshiftGroupIds(cycle_id:$cid) }`,
      { cid: Number(cid) }
    );
    const order = res.adminDayshiftGroupIds || [];
    const globalIndex = (groupPage - 1) * pageSize + idx;
    if (globalIndex < 0 || globalIndex >= order.length - 1) return;
    [order[globalIndex + 1], order[globalIndex]] = [
      order[globalIndex],
      order[globalIndex + 1],
    ];
    reorderGroups(order);
  }

  // user search combobox (fetch all when empty query)
  React.useEffect(() => {
    let active = true;
    if (!userOpen) {
      return;
    }
    setLoadingUsers(true);
    const q = userQuery.trim();
    const h = setTimeout(async () => {
      try {
        const res = await gql<{ adminUsers: AdminUser[] }>(
          `query($q:String){ adminUsers(q:$q){ id email display_name } }`,
          { q: q || undefined }
        );
        if (!active) return;
        setUserOptions(res.adminUsers || []);
      } catch (e: any) {
        if (!active) return;
        setUserOptions([]);
      } finally {
        if (active) setLoadingUsers(false);
      }
    }, 300);
    return () => {
      active = false;
      clearTimeout(h);
    };
  }, [userQuery, userOpen]);

  // client-side pagination helpers
  const paginatedCycles = React.useCallback(() => cycles, [cycles]);
  const paginatedGroups = React.useCallback(() => groups, [groups]);

  function userLabel(u: AdminUser | undefined) {
    if (!u) return "";
    return u.display_name && u.display_name.trim() !== ""
      ? u.display_name
      : u.email;
  }

  return (
    <div className="space-y-8">
      <div className="space-y-3">
        <h1 className="text-xl font-semibold">日直管理</h1>
        <h2 className="text-lg font-medium">サイクル</h2>
        <Card className="p-4 space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <div className="text-sm mb-1">開始日</div>
              <DatePicker value={begin} onChange={(v: string) => setBegin(v)} />
            </div>
            <div>
              <div className="text-sm mb-1">終了日</div>
              <DatePicker
                value={finish}
                onChange={(v: string) => setFinish(v)}
              />
            </div>
            <div>
              <div className="text-sm mb-1">説明</div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder="例: 2025年度 第1期"
              />
            </div>
          </div>
          <div className="flex justify-end">
            <Button onClick={createCycle} disabled={!begin || !finish || !desc}>
              サイクル作成
            </Button>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>開始</TableHead>
                <TableHead>終了</TableHead>
                <TableHead>説明</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingCycles ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="text-center text-sm text-muted-foreground"
                  >
                    読み込み中…
                  </TableCell>
                </TableRow>
              ) : cycles.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="text-center text-sm text-muted-foreground"
                  >
                    サイクルがありません
                  </TableCell>
                </TableRow>
              ) : (
                paginatedCycles().map((c) => (
                  <TableRow
                    key={c.id}
                    className={String(c.id) === cid ? "bg-muted/30" : ""}
                  >
                    <TableCell>{c.id}</TableCell>
                    <TableCell>{c.begin_date}</TableCell>
                    <TableCell>{c.finish_date}</TableCell>
                    <TableCell className="max-w-[240px] truncate">
                      {c.description}
                    </TableCell>
                    <TableCell className="text-right flex gap-2 justify-end">
                      <Button
                        variant="secondary"
                        onClick={() => setCid(String(c.id))}
                      >
                        選択
                      </Button>
                      <Button
                        variant="destructive"
                        onClick={() => deleteCycle(c.id)}
                      >
                        削除
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
          {cycleTotal > pageSize && (
            <div className="p-3">
              <Pagination>
                <PaginationContent>
                  <PaginationItem>
                    <PaginationPrevious
                      href="#"
                      onClick={(e) => {
                        e.preventDefault();
                        setCyclePage((p) => Math.max(1, p - 1));
                      }}
                    />
                  </PaginationItem>
                  {Array.from({ length: Math.ceil(cycleTotal / pageSize) }).map(
                    (_, i) => (
                      <PaginationItem key={i}>
                        <PaginationLink
                          href="#"
                          isActive={cyclePage === i + 1}
                          onClick={(e) => {
                            e.preventDefault();
                            setCyclePage(i + 1);
                          }}
                        >
                          {i + 1}
                        </PaginationLink>
                      </PaginationItem>
                    )
                  )}
                  <PaginationItem>
                    <PaginationNext
                      href="#"
                      onClick={(e) => {
                        e.preventDefault();
                        setCyclePage((p) =>
                          Math.min(Math.ceil(cycleTotal / pageSize), p + 1)
                        );
                      }}
                    />
                  </PaginationItem>
                </PaginationContent>
              </Pagination>
            </div>
          )}
        </Card>
      </div>
      {/* Prediction section (mod-based) */}
      <div className="space-y-3">
        <h2 className="text-lg font-medium">予測ペア（mod）</h2>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>#</TableHead>
                <TableHead>Cycle</TableHead>
                <TableHead>Group</TableHead>
                <TableHead>Members</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(() => {
                const selectedGroups = groups
                  .filter((g) => String(g.cycle_id) === (cid || autoCid))
                  .sort((a, b) => a.id - b.id);
                if (!(cid || autoCid) || selectedGroups.length === 0) {
                  return (
                    <TableRow>
                      <TableCell
                        colSpan={4}
                        className="text-center text-sm text-muted-foreground"
                      >
                        サイクルを選択し、グループを追加してください
                      </TableCell>
                    </TableRow>
                  );
                }
                const start = (predictPage - 1) * pageSize;
                const rows: React.ReactNode[] = [];
                for (let i = 0; i < pageSize; i++) {
                  const n = currentIndex + start + i; // n日目
                  const gidx = n % selectedGroups.length;
                  const g = selectedGroups[gidx];
                  rows.push(
                    <TableRow key={`pred-${n}`}>
                      <TableCell>{n}</TableCell>
                      <TableCell>{cid || autoCid}</TableCell>
                      <TableCell>{g.id}</TableCell>
                      <TableCell className="max-w-[420px] truncate">
                        {g.user_ids
                          .map((uid) => userLabel(userMap[uid]) || `ID:${uid}`)
                          .join(", ")}
                      </TableCell>
                    </TableRow>
                  );
                }
                return rows;
              })()}
            </TableBody>
          </Table>
          <div className="p-3">
            <Pagination>
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevious
                    href="#"
                    onClick={(e) => {
                      e.preventDefault();
                      setPredictPage((p) => Math.max(1, p - 1));
                    }}
                  />
                </PaginationItem>
                {Array.from({ length: 5 }).map((_, i) => (
                  <PaginationItem key={i}>
                    <PaginationLink
                      href="#"
                      isActive={predictPage === i + 1}
                      onClick={(e) => {
                        e.preventDefault();
                        setPredictPage(i + 1);
                      }}
                    >
                      {i + 1}
                    </PaginationLink>
                  </PaginationItem>
                ))}
                <PaginationItem>
                  <PaginationNext
                    href="#"
                    onClick={(e) => {
                      e.preventDefault();
                      setPredictPage((p) => p + 1);
                    }}
                  />
                </PaginationItem>
                <PaginationItem>
                  <PaginationEllipsis />
                </PaginationItem>
              </PaginationContent>
            </Pagination>
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <div className="text-sm mb-1">開始日（任意）</div>
              <DatePicker value={assignStart} onChange={setAssignStart} />
            </div>
            <div>
              <div className="text-sm mb-1">終了日（任意）</div>
              <DatePicker value={assignEnd} onChange={setAssignEnd} />
            </div>
            <div className="flex items-end justify-end gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  setAssignStart("");
                  setAssignEnd("");
                }}
              >
                クリア
              </Button>
              <Button
                onClick={() => loadAssignments()}
                disabled={loadingAssignments}
              >
                再読み込み
              </Button>
            </div>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>日付</TableHead>
                <TableHead>Cycle</TableHead>
                <TableHead>Users</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingAssignments ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="text-center text-sm text-muted-foreground"
                  >
                    読み込み中…
                  </TableCell>
                </TableRow>
              ) : assignments.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="text-center text-sm text-muted-foreground"
                  >
                    {cid || assignStart || assignEnd
                      ? "割り当てがありません"
                      : "サイクルまたは日付範囲を指定してください"}
                  </TableCell>
                </TableRow>
              ) : (
                assignments.map((a) => (
                  <TableRow key={`${a.date}-${a.id}`}>
                    <TableCell className="whitespace-nowrap">
                      {a.date}
                    </TableCell>
                    <TableCell>{a.cycle_id || "-"}</TableCell>
                    <TableCell className="max-w-[420px] truncate">
                      {a.user_ids
                        .map((uid) => userLabel(userMap[uid]) || `ID:${uid}`)
                        .join(", ")}
                    </TableCell>
                    <TableCell className="uppercase">
                      {a.status || "-"}
                    </TableCell>
                    <TableCell className="text-right flex gap-2 justify-end">
                      <Button
                        variant="secondary"
                        onClick={() => {
                          setEditAsgDate(a.date);
                          setEditAsgSelected(
                            a.user_ids
                              .map((id) => userMap[id])
                              .filter(Boolean) as AdminUser[]
                          );
                          setEditAsgOpen(true);
                        }}
                      >
                        メンバー編集
                      </Button>
                      <Button
                        variant="destructive"
                        onClick={async () => {
                          try {
                            await gql<{
                              adminDayshiftDeleteAssignment: boolean;
                            }>(
                              `mutation($d:String!){ adminDayshiftDeleteAssignment(date:$d) }`,
                              { d: a.date }
                            );
                            toast({ title: "削除しました" });
                            loadAssignments();
                          } catch (e: any) {
                            toast({
                              variant: "destructive",
                              title: "削除に失敗しました",
                              description: e?.message,
                            });
                          }
                        }}
                      >
                        削除
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </Card>
        {/* Shift controls */}
        <Card className="p-4 space-y-3">
          <div className="grid gap-3 sm:grid-cols-3 items-end">
            <div>
              <div className="text-sm mb-1">開始日</div>
              <DatePicker
                value={assignStart}
                onChange={(v: string) => setAssignStart(v)}
              />
            </div>
            <div className="sm:col-span-2 flex gap-2">
              <Button
                variant="outline"
                onClick={async () => {
                  if (!assignStart) return;
                  try {
                    await gql<{ adminDayshiftShift: boolean }>(
                      `mutation($s:String!){ adminDayshiftShift(start:$s, direction:"backward") }`,
                      { s: assignStart }
                    );
                    toast({ title: "1営業日、前へずらしました" });
                    loadAssignments();
                  } catch (e: any) {
                    toast({
                      variant: "destructive",
                      title: "操作に失敗しました",
                      description: e?.message,
                    });
                  }
                }}
              >
                前へ1営業日
              </Button>
              <Button
                onClick={async () => {
                  if (!assignStart) return;
                  try {
                    await gql<{ adminDayshiftShift: boolean }>(
                      `mutation($s:String!){ adminDayshiftShift(start:$s, direction:"forward") }`,
                      { s: assignStart }
                    );
                    toast({ title: "1営業日、後ろへずらしました" });
                    loadAssignments();
                  } catch (e: any) {
                    toast({
                      variant: "destructive",
                      title: "操作に失敗しました",
                      description: e?.message,
                    });
                  }
                }}
              >
                後ろへ1営業日
              </Button>
            </div>
          </div>
        </Card>
        {/* Edit Assignment Members Dialog */}
        <Dialog open={editAsgOpen} onOpenChange={setEditAsgOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{editAsgDate} のメンバーを編集</DialogTitle>
            </DialogHeader>
            <div className="space-y-2">
              <div className="flex flex-wrap gap-2">
                {editAsgSelected.map((u) => (
                  <span
                    key={u.id}
                    className="inline-flex items-center px-2 py-1 rounded bg-muted text-sm"
                  >
                    {userLabel(u)}
                    <button
                      className="ml-1 opacity-70 hover:opacity-100"
                      onClick={() =>
                        setEditAsgSelected((arr) =>
                          arr.filter((x) => x.id !== u.id)
                        )
                      }
                    >
                      <XIcon className="h-3 w-3" />
                    </button>
                  </span>
                ))}
              </div>
              <Popover open={userOpen} onOpenChange={setUserOpen}>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="justify-between w-full">
                    追加するユーザーを検索
                    <ChevronsUpDownIcon className="ml-2 h-4 w-4 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="p-0 w-[500px]">
                  <Command>
                    <CommandInput
                      placeholder="名前またはメールで検索..."
                      value={userQuery}
                      onValueChange={setUserQuery}
                    />
                    <CommandList>
                      <CommandEmpty>
                        {loadingUsers
                          ? "検索中…"
                          : "一致するユーザーがいません"}
                      </CommandEmpty>
                      <CommandGroup>
                        {userOptions.map((u) => (
                          <CommandItem
                            key={u.id}
                            value={String(u.id)}
                            onSelect={() => {
                              setEditAsgSelected((arr) =>
                                arr.find((x) => x.id === u.id)
                                  ? arr
                                  : [...arr, u]
                              );
                            }}
                          >
                            <CheckIcon
                              className={cn(
                                "mr-2 h-4 w-4",
                                editAsgSelected.find((x) => x.id === u.id)
                                  ? "opacity-100"
                                  : "opacity-0"
                              )}
                            />
                            <span className="truncate">{userLabel(u)}</span>
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>
            <DialogFooter>
              <Button variant="secondary" onClick={() => setEditAsgOpen(false)}>
                キャンセル
              </Button>
              <Button
                onClick={async () => {
                  if (!editAsgDate) return;
                  try {
                    await gql<{
                      adminDayshiftUpdateAssignmentMembers: boolean;
                    }>(
                      `mutation($d:String!,$ids:[Int!]!){ adminDayshiftUpdateAssignmentMembers(date:$d, user_ids:$ids) }`,
                      { d: editAsgDate, ids: editAsgSelected.map((u) => u.id) }
                    );
                    setEditAsgOpen(false);
                    loadAssignments();
                  } catch (e: any) {
                    toast({
                      variant: "destructive",
                      title: "更新に失敗しました",
                      description: e?.message,
                    });
                  }
                }}
              >
                保存
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
      {/* Index config control */}
      <div className="space-y-3">
        <h2 className="text-lg font-medium">インデックス値</h2>
        <p>
          操作値は(n%グループ数)で決定されていますが、この現在のインデックス値を設定します。この操作は慎重に行ってください。
        </p>
        <Card className="p-4 flex items-center gap-3">
          <Input
            type="number"
            value={String(currentIndex)}
            onChange={(e) =>
              setCurrentIndex(parseInt(e.target.value || "0", 10))
            }
            className="w-28"
          />
          <Button
            onClick={async () => {
              try {
                setSavingIndex(true);
                await gql<{ adminUpdateDayShiftIndex: boolean }>(
                  `mutation($v:Int!){ adminUpdateDayShiftIndex(index:$v) }`,
                  { v: currentIndex }
                );
              } finally {
                setSavingIndex(false);
              }
            }}
            disabled={savingIndex}
          >
            更新
          </Button>
        </Card>
      </div>
      {/* Current cycle id control */}
      <div className="space-y-3">
        <h2 className="text-lg font-medium">現在のサイクルID</h2>
        <p>自動選択される現在のサイクルIDをここで設定できます。</p>
        <Card className="p-4 flex items-center gap-3">
          <Input
            type="number"
            value={currentCycleId}
            onChange={(e) => setCurrentCycleId(e.target.value)}
            className="w-28"
            placeholder="例: 12"
          />
          <Button
            onClick={async () => {
              try {
                setSavingCycleId(true);
                await gql<{ adminSetDayShiftCurrentCycle: boolean }>(
                  `mutation($cid:Int!){ adminSetDayShiftCurrentCycle(cycle_id:$cid) }`,
                  { cid: parseInt(currentCycleId || "0", 10) }
                );
                // reflect in autoCid for predictions
                if (!cid) setAutoCid(currentCycleId);
              } finally {
                setSavingCycleId(false);
              }
            }}
            disabled={savingCycleId || currentCycleId.trim() === ""}
          >
            更新
          </Button>
        </Card>
      </div>
      <div className="space-y-3">
        <h2 className="text-lg font-medium">グループ</h2>
        <Card className="p-4 space-y-4">
          <div className="grid gap-4 sm:grid-cols-3">
            {/* Cycle combobox (search by description) */}
            <div className="space-y-1">
              <div className="text-sm">サイクル（説明で検索）</div>
              <Popover open={cycleOpen} onOpenChange={setCycleOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={cycleOpen}
                    className="justify-between w-full"
                  >
                    {cid
                      ? cycles.find((c) => String(c.id) === cid)?.description ||
                        `ID: ${cid}`
                      : autoCid
                      ? (cycles.find((c) => String(c.id) === autoCid)
                          ?.description || `ID: ${autoCid}`) + "（自動）"
                      : "サイクルを選択"}
                    <ChevronsUpDownIcon className="ml-2 h-4 w-4 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="p-0 w-[360px]">
                  <Command>
                    <CommandInput
                      placeholder="説明で検索..."
                      value={cycleFilter}
                      onValueChange={setCycleFilter}
                    />
                    <CommandList>
                      <CommandEmpty>一致するサイクルがありません</CommandEmpty>
                      <CommandGroup>
                        {cycles
                          .filter((c) =>
                            (c.description || "")
                              .toLowerCase()
                              .includes(cycleFilter.toLowerCase())
                          )
                          .map((c) => (
                            <CommandItem
                              key={c.id}
                              value={String(c.id)}
                              onSelect={() => {
                                setCid(String(c.id));
                                setCycleOpen(false);
                              }}
                            >
                              <CheckIcon
                                className={cn(
                                  "mr-2 h-4 w-4",
                                  String(c.id) === cid
                                    ? "opacity-100"
                                    : "opacity-0"
                                )}
                              />
                              <span className="truncate">
                                {c.description}（{c.begin_date}〜{c.finish_date}
                                ）
                              </span>
                            </CommandItem>
                          ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>

            {/* User combobox (search by display name/email) */}
            <div className="space-y-1 sm:col-span-2">
              <div className="text-sm">ユーザーを検索して追加（複数可）</div>
              <div className="flex flex-wrap gap-2">
                {selectedUsers.map((u) => (
                  <span
                    key={u.id}
                    className="inline-flex items-center px-2 py-1 rounded bg-muted text-sm"
                  >
                    {userLabel(u)}
                    <button
                      className="ml-1 opacity-70 hover:opacity-100"
                      onClick={() =>
                        setSelectedUsers((arr) =>
                          arr.filter((x) => x.id !== u.id)
                        )
                      }
                    >
                      <XIcon className="h-3 w-3" />
                    </button>
                  </span>
                ))}
              </div>
              <Popover open={userOpen} onOpenChange={setUserOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={userOpen}
                    className="justify-between w-full"
                  >
                    {loadingUsers ? "検索中…" : "追加するユーザーを検索"}
                    <ChevronsUpDownIcon className="ml-2 h-4 w-4 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="p-0 w-[500px]">
                  <Command>
                    <CommandInput
                      placeholder="名前またはメールで検索..."
                      value={userQuery}
                      onValueChange={setUserQuery}
                    />
                    <CommandList>
                      <CommandEmpty>
                        {loadingUsers
                          ? "検索中…"
                          : userQuery.trim() === ""
                          ? "ユーザーがいません"
                          : "一致するユーザーがいません"}
                      </CommandEmpty>
                      <CommandGroup>
                        {userOptions.map((u) => (
                          <CommandItem
                            key={u.id}
                            value={String(u.id)}
                            onSelect={() => {
                              setSelectedUsers((arr) => {
                                if (arr.find((x) => x.id === u.id)) return arr;
                                return [...arr, u];
                              });
                            }}
                          >
                            <CheckIcon
                              className={cn(
                                "mr-2 h-4 w-4",
                                selectedUsers.find((x) => x.id === u.id)
                                  ? "opacity-100"
                                  : "opacity-0"
                              )}
                            />
                            <span className="truncate">{userLabel(u)}</span>
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>
          </div>
          <div className="flex justify-end">
            <Button
              onClick={createGroup}
              disabled={!cid || selectedUsers.length === 0}
            >
              グループ追加
            </Button>
            <Button
              variant="outline"
              className="ml-2"
              onClick={() => setBulkOpen(true)}
              disabled={!cid}
            >
              CSV一括追加
            </Button>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Cycle</TableHead>
                <TableHead>Users</TableHead>
                <TableHead>Position</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingGroups ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-sm text-muted-foreground"
                  >
                    読み込み中…
                  </TableCell>
                </TableRow>
              ) : groups.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-sm text-muted-foreground"
                  >
                    グループがありません
                  </TableCell>
                </TableRow>
              ) : (
                paginatedGroups().map((g, idx) => (
                  <TableRow key={g.id}>
                    <TableCell>{g.id}</TableCell>
                    <TableCell>{g.cycle_id}</TableCell>
                    <TableCell className="max-w-[420px] truncate">
                      {g.user_ids
                        .map((uid) => userLabel(userMap[uid]) || `ID:${uid}`)
                        .join(", ")}
                    </TableCell>
                    <TableCell>{g.position}</TableCell>
                    <TableCell className="text-right flex gap-2 justify-end">
                      <Button
                        variant="secondary"
                        size="icon"
                        title="編集"
                        onClick={() => {
                          setEditGroup(g);
                          setEditSelected(
                            g.user_ids
                              .map((id) => userMap[id])
                              .filter(Boolean) as AdminUser[]
                          );
                          setEditOpen(true);
                        }}
                      >
                        <PencilIcon className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="outline"
                        size="icon"
                        onClick={() => moveUp(idx)}
                        title="上へ"
                      >
                        <ArrowUpIcon className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="outline"
                        size="icon"
                        onClick={() => moveDown(idx)}
                        title="下へ"
                      >
                        <ArrowDownIcon className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="destructive"
                        onClick={() => deleteGroup(g.id)}
                      >
                        削除
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
          {groupTotal > pageSize && (
            <div className="p-3">
              <Pagination>
                <PaginationContent>
                  <PaginationItem>
                    <PaginationPrevious
                      href="#"
                      onClick={(e) => {
                        e.preventDefault();
                        setGroupPage((p) => Math.max(1, p - 1));
                      }}
                    />
                  </PaginationItem>
                  {Array.from({ length: Math.ceil(groupTotal / pageSize) }).map(
                    (_, i) => (
                      <PaginationItem key={i}>
                        <PaginationLink
                          href="#"
                          isActive={groupPage === i + 1}
                          onClick={(e) => {
                            e.preventDefault();
                            setGroupPage(i + 1);
                          }}
                        >
                          {i + 1}
                        </PaginationLink>
                      </PaginationItem>
                    )
                  )}
                  <PaginationItem>
                    <PaginationNext
                      href="#"
                      onClick={(e) => {
                        e.preventDefault();
                        setGroupPage((p) =>
                          Math.min(Math.ceil(groupTotal / pageSize), p + 1)
                        );
                      }}
                    />
                  </PaginationItem>
                </PaginationContent>
              </Pagination>
            </div>
          )}
        </Card>
        <Dialog open={editOpen} onOpenChange={setEditOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>メンバーを編集</DialogTitle>
            </DialogHeader>
            <div className="space-y-2">
              <div className="flex flex-wrap gap-2">
                {editSelected.map((u) => (
                  <span
                    key={u.id}
                    className="inline-flex items-center px-2 py-1 rounded bg-muted text-sm"
                  >
                    {userLabel(u)}
                    <button
                      className="ml-1 opacity-70 hover:opacity-100"
                      onClick={() =>
                        setEditSelected((arr) =>
                          arr.filter((x) => x.id !== u.id)
                        )
                      }
                    >
                      <XIcon className="h-3 w-3" />
                    </button>
                  </span>
                ))}
              </div>
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="justify-between w-full">
                    追加するユーザーを検索
                    <ChevronsUpDownIcon className="ml-2 h-4 w-4 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="p-0 w-[500px]">
                  <Command>
                    <CommandInput
                      placeholder="名前またはメールで検索..."
                      value={userQuery}
                      onValueChange={setUserQuery}
                    />
                    <CommandList>
                      <CommandEmpty>一致するユーザーがいません</CommandEmpty>
                      <CommandGroup>
                        {userOptions.map((u) => (
                          <CommandItem
                            key={u.id}
                            value={String(u.id)}
                            onSelect={() => {
                              setEditSelected((arr) =>
                                arr.find((x) => x.id === u.id)
                                  ? arr
                                  : [...arr, u]
                              );
                            }}
                          >
                            <CheckIcon
                              className={cn(
                                "mr-2 h-4 w-4",
                                editSelected.find((x) => x.id === u.id)
                                  ? "opacity-100"
                                  : "opacity-0"
                              )}
                            />
                            <span className="truncate">{userLabel(u)}</span>
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>
            <DialogFooter>
              <Button variant="secondary" onClick={() => setEditOpen(false)}>
                キャンセル
              </Button>
              <Button
                onClick={async () => {
                  if (!editGroup) return;
                  try {
                    await gql<{ adminDayshiftUpdateGroupMembers: boolean }>(
                      `mutation($gid:Int!,$ids:[Int!]!){ adminDayshiftUpdateGroupMembers(group_id:$gid,user_ids:$ids) }`,
                      { gid: editGroup.id, ids: editSelected.map((u) => u.id) }
                    );
                    setEditOpen(false);
                    loadGroups();
                  } catch (e: any) {
                    toast({
                      variant: "destructive",
                      title: "更新に失敗しました",
                      description: e?.message,
                    });
                  }
                }}
              >
                保存
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
        {/* Bulk add groups via CSV */}
        <Dialog open={bulkOpen} onOpenChange={setBulkOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>CSVでグループ一括追加</DialogTitle>
            </DialogHeader>
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                1行につき1グループを追加します。各行は「ユーザーIDをカンマ区切り」で指定してください。
                例: <code>12,34</code> や <code>5,8,9</code>
              </p>
              <Textarea
                value={bulkCSV}
                onChange={(e) => setBulkCSV(e.target.value)}
                placeholder={"例:\n12,34\n5,8,9"}
                rows={8}
              />
            </div>
            <DialogFooter>
              <Button variant="secondary" onClick={() => setBulkOpen(false)}>
                キャンセル
              </Button>
              <Button
                disabled={!cid || bulkLoading || bulkCSV.trim() === ""}
                onClick={async () => {
                  if (!cid) return;
                  setBulkLoading(true);
                  try {
                    const lines = bulkCSV.split(/\r?\n/);
                    let created = 0;
                    const errors: string[] = [];
                    for (let i = 0; i < lines.length; i++) {
                      const raw = lines[i].trim();
                      if (!raw) continue;
                      const ids = raw
                        .split(",")
                        .map((s) => parseInt(s.trim(), 10))
                        .filter((n) => Number.isFinite(n) && n > 0);
                      // dedupe while keeping order
                      const seen = new Set<number>();
                      const uniq: number[] = [];
                      for (const n of ids) {
                        if (!seen.has(n)) {
                          seen.add(n);
                          uniq.push(n);
                        }
                      }
                      if (uniq.length === 0) {
                        errors.push(`行${i + 1}: ユーザーIDが不正です`);
                        continue;
                      }
                      try {
                        await gql<{ adminDayshiftCreateGroup: { id: number } }>(
                          `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftCreateGroup(cycle_id:$cid, user_ids:$ids){ id } }`,
                          { cid: Number(cid), ids: uniq }
                        );
                        created++;
                      } catch (e: any) {
                        errors.push(
                          `行${i + 1}: ${e?.message || "作成に失敗"}`
                        );
                      }
                    }
                    if (created > 0) {
                      toast({
                        title: "一括追加が完了しました",
                        description: `${created}件のグループを追加しました${
                          errors.length ? `（エラー ${errors.length} 行）` : ""
                        }`,
                      });
                      setBulkOpen(false);
                      setBulkCSV("");
                      loadGroups();
                    }
                    if (errors.length > 0) {
                      toast({
                        variant: "destructive",
                        title: "一部で失敗しました",
                        description:
                          errors.slice(0, 5).join("\n") +
                          (errors.length > 5
                            ? `\n…ほか ${errors.length - 5} 件`
                            : ""),
                      });
                    }
                  } finally {
                    setBulkLoading(false);
                  }
                }}
              >
                追加する
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  );
}

// Inline simple DatePicker built with Popover + Calendar (shadcn pattern)
function DatePicker({
  value,
  onChange,
  placeholder,
}: {
  value?: string;
  onChange?: (v: string) => void;
  placeholder?: string;
}) {
  const [open, setOpen] = React.useState(false);
  const date = React.useMemo(() => {
    if (!value) return undefined as Date | undefined;
    const [y, m, d] = value.split("-").map((v) => parseInt(v, 10));
    if (!y || !m || !d) return undefined;
    return new Date(y, m - 1, d);
  }, [value]);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          data-empty={!date}
          className="data-[empty=true]:text-muted-foreground w-[220px] justify-start text-left font-normal"
          type="button"
        >
          <CalendarIcon className="mr-2 h-4 w-4" />
          {date ? (
            date.toLocaleDateString()
          ) : (
            <span>{placeholder || "日付を選択"}</span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0">
        <Calendar
          mode="single"
          selected={date}
          onSelect={(d: Date | undefined) => {
            const s = d
              ? `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(
                  2,
                  "0"
                )}-${String(d.getDate()).padStart(2, "0")}`
              : "";
            onChange && onChange(s);
            setOpen(false);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
