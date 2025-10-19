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

export default function AdminDayShift() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [cycles, setCycles] = React.useState<Cycle[]>([]);
  const [cid, setCid] = React.useState<string>("");
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

  // edit members dialog state
  const [editOpen, setEditOpen] = React.useState(false);
  const [editGroup, setEditGroup] = React.useState<Group | null>(null);
  const [editSelected, setEditSelected] = React.useState<AdminUser[]>([]);

  async function loadCycles() {
    try {
      setLoadingCycles(true);
      const d = await gql<{
        adminDayshiftCyclesPaginated: { items: Cycle[]; total: number };
      }>(
        `query($page:Int,$size:Int){ adminDayshiftCyclesPaginated(page:$page,page_size:$size){ items{ id begin_date finish_date description } total } }`,
        { page: cyclePage, size: pageSize }
      );
      setCycles(d.adminDayshiftCyclesPaginated?.items || []);
      setCycleTotal(d.adminDayshiftCyclesPaginated?.total || 0);
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
    if (!cid) {
      setGroups([]);
      return;
    }
    try {
      setLoadingGroups(true);
      const d = await gql<{
        adminDayshiftGroupsPaginated: { items: Group[]; total: number };
      }>(
        `query($cid:Int!,$page:Int,$size:Int){ adminDayshiftGroupsPaginated(cycle_id:$cid,page:$page,page_size:$size){ items{ id cycle_id position user_ids } total } }`,
        { cid: Number(cid), page: groupPage, size: pageSize }
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
  React.useEffect(() => {
    if (token) loadCycles();
  }, [token, cyclePage]);
  React.useEffect(() => {
    setGroupPage(1);
  }, [cid]);
  React.useEffect(() => {
    loadGroups();
  }, [cid, groupPage]);

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
