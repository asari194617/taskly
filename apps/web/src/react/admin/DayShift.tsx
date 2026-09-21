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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
} from "lucide-react";
import { toast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { DatePicker } from "@/features/admin-day-shift/DatePicker";
import { getToken, requireAuth } from "@/lib/auth-guard";
import {
  createCycle as createCycleRequest,
  createGroup as createGroupRequest,
  deleteCycle as deleteCycleRequest,
  deleteGroup as deleteGroupRequest,
  loadCurrentCycleId,
  loadCycles as loadCyclesRequest,
  loadDayShiftIndex,
  loadGroupIds,
  loadGroups as loadGroupsRequest,
  loadUsersByIds,
  reorderGroups as reorderGroupsRequest,
  searchAdminUsers,
  setCurrentCycle,
  updateDayShiftIndex,
  updateGroupMembers,
  type AdminUser,
  type Cycle,
  type Group,
} from "@/features/admin-day-shift/api";
type Assignment = {
  id: number;
  date: string; // YYYY-MM-DD
  cycle_id: number;
  status: string;
  user_ids: number[];
};

export default function AdminDayShift() {
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
  async function loadCycles() {
    try {
      setLoadingCycles(true);
      const d = await loadCyclesRequest(cyclePage, pageSize);
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
      const d = await loadGroupsRequest(Number(active), groupPage, pageSize);
      const arr = d.adminDayshiftGroupsPaginated?.items || [];
      setGroupTotal(d.adminDayshiftGroupsPaginated?.total || 0);
      setGroups(arr);
      // fetch user details for rendering
      const ids = Array.from(new Set(arr.flatMap((g) => g.user_ids)));
      if (ids.length > 0) {
        const users = await loadUsersByIds(ids);
        const um: Record<number, AdminUser> = {};
        for (const u of users) um[u.id] = u;
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
  React.useEffect(() => {
    if (getToken()) {
      loadCycles();
      loadDayShiftIndex()
        .then(setCurrentIndex)
        .catch(() => setCurrentIndex(0));
      loadCurrentCycleId()
        .then((v) => {
          setCurrentCycleId(v ? String(v) : "");
        })
        .catch(() => setCurrentCycleId(""));
    } else {
      requireAuth();
    }
  }, [cyclePage]);
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
  // assignments UI removed: rotations now handled purely via index+groups

  async function createCycle() {
    try {
      await createCycleRequest(begin, finish, desc);
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
      await deleteCycleRequest(id);
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
      await createGroupRequest(Number(cid), selectedUsers.map((u) => u.id));
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
      await deleteGroupRequest(Number(cid), id);
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
      await reorderGroupsRequest(Number(cid), newOrder);
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
    const order = await loadGroupIds(Number(cid));
    const globalIndex = (groupPage - 1) * pageSize + idx;
    if (globalIndex <= 0 || globalIndex >= order.length) return;
    [order[globalIndex - 1], order[globalIndex]] = [
      order[globalIndex],
      order[globalIndex - 1],
    ];
    reorderGroups(order);
  }
  async function moveDown(idx: number) {
    const order = await loadGroupIds(Number(cid));
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
        const users = await searchAdminUsers(q);
        if (!active) return;
        setUserOptions(users);
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
        <h2 className="text-lg font-medium">自分以降の順番予測</h2>
        <Alert>
          <AlertTitle>インデックスとグループの対応について</AlertTitle>
          <AlertDescription>
            現在のインデックス値はサーバー側と同じく0始まりで扱われます。
            下の表は「現在のインデックス値の担当者」を1行目として、そこから先の順番を表示します。
            各行の担当グループは、内部インデックス値
            <code>(currentIndex + オフセット)</code>に対して
            <code>(内部インデックス値 % グループ数)</code>で決まります。
            ページを進めると内部インデックス値も進み、それに応じてグループやメンバー、
            サイクルIDも順番どおりに変化します。
          </AlertDescription>
        </Alert>
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
                  .sort((a, b) => a.position - b.position || a.id - b.id);
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
                  const offset = start + i; // 「自分から何番目か」を表すオフセット
                  const logicalIndex = currentIndex + offset; // サーバー側と同じ0始まりのインデックス

                  if (logicalIndex < 0) continue;

                  const totalGroups = selectedGroups.length;
                  const cycleOffset = Math.floor(logicalIndex / totalGroups);
                  const groupIndexInCycle =
                    ((logicalIndex % totalGroups) + totalGroups) % totalGroups;
                  const g = selectedGroups[groupIndexInCycle];

                  const cycleDisplayId = (() => {
                    const base = Number(cid || autoCid) || 0;
                    return base ? base + cycleOffset : base;
                  })();
                  rows.push(
                    <TableRow key={`pred-${logicalIndex}`}>
                      <TableCell>{offset + 1}</TableCell>
                      <TableCell>{cycleDisplayId || "-"}</TableCell>
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
                await updateDayShiftIndex(currentIndex);
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
                await setCurrentCycle(parseInt(currentCycleId || "0", 10));
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
                    await updateGroupMembers(
                      editGroup.id,
                      editSelected.map((u) => u.id),
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
                        await createGroupRequest(Number(cid), uniq);
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
