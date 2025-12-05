"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreVertical } from "lucide-react";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";

type AUser = {
  id: number;
  email: string;
  display_name?: string | null;
  notification_lang?: string | null;
  email_login_enabled?: boolean | null;
  roles: string[];
  is_blocked: boolean;
};

export default function AdminUsers() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [q, setQ] = React.useState("");
  const [items, setItems] = React.useState<AUser[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [page, setPage] = React.useState(1);
  const [pageSize] = React.useState(20);
  const [total, setTotal] = React.useState(0);
  const [sortBy, setSortBy] = React.useState<
    "id" | "email" | "display_name" | "created_at" | null
  >(null);
  const [sortDir, setSortDir] = React.useState<"asc" | "desc" | null>(null);

  // Dialog/Drawer responsive states
  const [openEdit, setOpenEdit] = React.useState(false);
  const [editTarget, setEditTarget] = React.useState<AUser | null>(null);
  const [editEmail, setEditEmail] = React.useState("");
  const [editDisplayName, setEditDisplayName] = React.useState("");
  const [editLang, setEditLang] = React.useState("ja");
  const [editEmailLogin, setEditEmailLogin] = React.useState(true);

  const [openCreate, setOpenCreate] = React.useState(false);
  const [newEmail, setNewEmail] = React.useState("");
  const [newDisplayName, setNewDisplayName] = React.useState("");
  const [newLang, setNewLang] = React.useState("ja");

  const [openBulk, setOpenBulk] = React.useState(false);
  const [bulkCsv, setBulkCsv] = React.useState("");

  const [openDelete, setOpenDelete] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<AUser | null>(null);
  const [deleteHard, setDeleteHard] = React.useState(false);
  const [deleteConfirm, setDeleteConfirm] = React.useState("");

  // Block user dialog
  const [openBlock, setOpenBlock] = React.useState(false);
  const [blockTarget, setBlockTarget] = React.useState<AUser | null>(null);
  const [blockReason, setBlockReason] = React.useState("");

  // Notification settings dialog/drawer
  const isMobile = useIsMobile();
  const [openNotif, setOpenNotif] = React.useState(false);
  const [notifTarget, setNotifTarget] = React.useState<AUser | null>(null);
  const [notifEmail, setNotifEmail] = React.useState(true);
  const [notifPush, setNotifPush] = React.useState(false);
  // Type prefs
  const [typeTask, setTypeTask] = React.useState(true);
  const [typeDayShift, setTypeDayShift] = React.useState(true);
  // Devices
  type Device = {
    id: number;
    user_id: number;
    platform: string;
    name?: string | null;
    payload?: string | null;
    is_enabled: boolean;
    created_at: string;
    updated_at: string;
  };
  const [devices, setDevices] = React.useState<Device[]>([]);
  const [newDevPlatform, setNewDevPlatform] = React.useState("WEBPUSH");
  const [newDevName, setNewDevName] = React.useState("");
  const [newDevPayload, setNewDevPayload] = React.useState("");
  const [newDevEnabled, setNewDevEnabled] = React.useState(true);

  // Custom role dialogs
  const [openCustomAdd, setOpenCustomAdd] = React.useState(false);
  const [openCustomRemove, setOpenCustomRemove] = React.useState(false);
  const [customTarget, setCustomTarget] = React.useState<AUser | null>(null);
  const [customRole, setCustomRole] = React.useState("");

  async function load() {
    setLoading(true);
    try {
      const d = await gql<{
        adminUsersPaginated: { items: AUser[]; total: number };
      }>(
        `query($q:String,$sb:String,$sd:String,$p:Int,$ps:Int){ adminUsersPaginated(q:$q,sort_by:$sb,sort_dir:$sd,page:$p,page_size:$ps){ items{ id email display_name notification_lang email_login_enabled roles is_blocked } total } }`,
        {
          q: q || undefined,
          sb: sortBy || undefined,
          sd: sortDir || undefined,
          p: page,
          ps: pageSize,
        }
      );
      setItems(d.adminUsersPaginated?.items || []);
      setTotal(d.adminUsersPaginated?.total || 0);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoading(false);
    }
  }
  React.useEffect(() => {
    load();
  }, [token]);
  React.useEffect(() => {
    load();
  }, [page, sortBy, sortDir]);
  // optional: reload when query cleared (to avoid sticky empty state)
  React.useEffect(() => {
    if (q.trim() === "") {
      setPage(1);
      load();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q]);

  async function addRole(id: number, role: string) {
    try {
      await gql<{ adminUserAddRole: boolean }>(
        `mutation($id:Int!,$r:String!){ adminUserAddRole(user_id:$id, role:$r) }`,
        { id, r: role }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "付与に失敗しました",
        description: e?.message,
      });
    }
  }
  async function removeRole(id: number, role: string) {
    try {
      await gql<{ adminUserRemoveRole: boolean }>(
        `mutation($id:Int!,$r:String!){ adminUserRemoveRole(user_id:$id, role:$r) }`,
        { id, r: role }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }
  function openBlockDialog(u: AUser) {
    setBlockTarget(u);
    setBlockReason("");
    setOpenBlock(true);
  }
  async function submitBlock() {
    if (!blockTarget) return;
    try {
      await gql<{ adminUserBlock: boolean }>(
        `mutation($id:Int!,$reason:String){ adminUserBlock(user_id:$id, reason:$reason) }`,
        { id: blockTarget.id, reason: blockReason || undefined }
      );
      setOpenBlock(false);
      setBlockTarget(null);
      setBlockReason("");
      load();
      toast({ title: "ブロックしました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "ブロックに失敗しました",
        description: e?.message,
      });
    }
  }
  async function unblockUser(id: number) {
    try {
      await gql<{ adminUserUnblock: boolean }>(
        `mutation($id:Int!){ adminUserUnblock(user_id:$id) }`,
        { id }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "解除に失敗しました",
        description: e?.message,
      });
    }
  }

  function toggleSort(key: "id" | "email" | "display_name" | "created_at") {
    if (sortBy !== key) {
      setSortBy(key);
      setSortDir("asc");
      setPage(1);
      return;
    }
    if (sortDir === "asc") setSortDir("desc");
    else if (sortDir === "desc") {
      setSortBy(null);
      setSortDir(null);
    } else setSortDir("asc");
    setPage(1);
  }

  function sortLabel(key: "id" | "email" | "display_name" | "created_at") {
    if (sortBy !== key) return "";
    return sortDir === "asc" ? " ▲" : sortDir === "desc" ? " ▼" : "";
  }

  function openEditDialog(u: AUser) {
    setEditTarget(u);
    setEditEmail(u.email || "");
    setEditDisplayName(u.display_name || "");
    setEditLang((u.notification_lang as string) || "ja");
    setEditEmailLogin(!!u.email_login_enabled);
    setOpenEdit(true);
  }

  async function submitEdit() {
    if (!editTarget) return;
    try {
      await gql<{ adminUserUpdate: boolean }>(
        `mutation($id:Int!,$email:String,$dn:String,$lang:String,$ema:Boolean){ adminUserUpdate(user_id:$id, email:$email, display_name:$dn, notification_lang:$lang, email_login_enabled:$ema) }`,
        {
          id: editTarget.id,
          email: editEmail || undefined,
          dn: editDisplayName || undefined,
          lang: editLang || undefined,
          ema: editEmailLogin,
        }
      );
      setOpenEdit(false);
      load();
      toast({ title: "更新しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }

  // Shared content components (for responsive Dialog/Drawer)
  function EditUserForm() {
    return (
      <div className="space-y-3">
        <div>
          <div className="text-sm mb-1">メール</div>
          <Input
            value={editEmail}
            onChange={(e) => setEditEmail(e.target.value)}
          />
        </div>
        <div>
          <div className="text-sm mb-1">表示名</div>
          <Input
            value={editDisplayName}
            onChange={(e) => setEditDisplayName(e.target.value)}
          />
        </div>
        <div>
          <div className="text-sm mb-1">表示言語</div>
          <Select value={editLang} onValueChange={setEditLang}>
            <SelectTrigger>
              <SelectValue placeholder="言語" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ja">日本語</SelectItem>
              <SelectItem value="en">English</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <Switch
            checked={editEmailLogin}
            onCheckedChange={setEditEmailLogin}
            id="email-login"
          />
          <label htmlFor="email-login" className="text-sm">
            メールログインを許可
          </label>
        </div>
      </div>
    );
  }

  function NotifSettingsContent() {
    return (
      <div className="space-y-4">
        <div className="text-sm text-muted-foreground">
          対象: {notifTarget?.email || "-"}
        </div>
        <div className="flex items-center gap-2">
          <Switch
            id="notif-email2"
            checked={notifEmail}
            onCheckedChange={setNotifEmail}
          />
          <label htmlFor="notif-email2" className="text-sm">
            メールで配信
          </label>
        </div>
        <div className="flex items-center gap-2">
          <Switch
            id="notif-push2"
            checked={notifPush}
            onCheckedChange={setNotifPush}
          />
          <label htmlFor="notif-push2" className="text-sm">
            プッシュ通知で配信
          </label>
        </div>
        <div className="pt-2">
          <div className="text-sm font-medium mb-2">通知種類</div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <Switch
                id="type-task"
                checked={typeTask}
                onCheckedChange={setTypeTask}
              />
              <label htmlFor="type-task" className="text-sm">
                課題通知（TASK）
              </label>
            </div>
            <div className="flex items-center gap-2">
              <Switch
                id="type-dayshift"
                checked={typeDayShift}
                onCheckedChange={setTypeDayShift}
              />
              <label htmlFor="type-dayshift" className="text-sm">
                日直通知（DAYSHIFT）
              </label>
            </div>
          </div>
        </div>
        <div className="pt-4 space-y-3">
          <div className="text-sm font-medium">デバイス一覧</div>
          <div className="border rounded">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>プラットフォーム</TableHead>
                  <TableHead>名前</TableHead>
                  <TableHead>有効</TableHead>
                  <TableHead>更新</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {devices.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={5}
                      className="text-sm text-muted-foreground"
                    >
                      デバイスがありません
                    </TableCell>
                  </TableRow>
                ) : (
                  devices.map((d) => (
                    <TableRow key={d.id}>
                      <TableCell>{d.platform}</TableCell>
                      <TableCell>{d.name || "-"}</TableCell>
                      <TableCell>
                        <Switch
                          checked={d.is_enabled}
                          onCheckedChange={(v) => toggleDeviceEnabled(d.id, v)}
                        />
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {new Date(d.updated_at).toLocaleString()}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          className="text-destructive"
                          onClick={() => deleteDevice(d.id)}
                        >
                          削除
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-5 gap-2 items-end">
            <div>
              <div className="text-sm mb-1">プラットフォーム</div>
              <Select value={newDevPlatform} onValueChange={setNewDevPlatform}>
                <SelectTrigger>
                  <SelectValue placeholder="選択" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="EMAIL">EMAIL</SelectItem>
                  <SelectItem value="WEBPUSH">WEBPUSH</SelectItem>
                  <SelectItem value="IOS">IOS</SelectItem>
                  <SelectItem value="ANDROID">ANDROID</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <div className="text-sm mb-1">名前</div>
              <Input
                value={newDevName}
                onChange={(e) => setNewDevName(e.target.value)}
                placeholder="任意"
              />
            </div>
            <div className="md:col-span-2">
              <div className="text-sm mb-1">Payload(JSON)</div>
              <Input
                value={newDevPayload}
                onChange={(e) => setNewDevPayload(e.target.value)}
                placeholder='{"endpoint":"..."}'
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                checked={newDevEnabled}
                onCheckedChange={setNewDevEnabled}
                id="newDevEnabled"
              />
              <label htmlFor="newDevEnabled" className="text-sm">
                有効
              </label>
            </div>
            <div className="md:col-span-5 text-right">
              <Button onClick={createDevice}>デバイス追加</Button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  async function submitCreate() {
    if (!newEmail.trim()) {
      toast({ variant: "destructive", title: "メールは必須です" });
      return;
    }
    try {
      await gql<{ adminUserCreate: { id: number } }>(
        `mutation($email:String!,$dn:String,$lang:String){ adminUserCreate(email:$email, display_name:$dn, notification_lang:$lang){ id } }`,
        {
          email: newEmail.trim(),
          dn: newDisplayName || undefined,
          lang: newLang || undefined,
        }
      );
      setOpenCreate(false);
      setNewEmail("");
      setNewDisplayName("");
      setNewLang("ja");
      setPage(1);
      load();
      toast({ title: "作成しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }

  async function submitBulk() {
    if (!bulkCsv.trim()) {
      toast({ variant: "destructive", title: "CSVを入力してください" });
      return;
    }
    try {
      const res = await gql<{
        adminUserBulkCreate: { created: number; errors: string[] };
      }>(
        `mutation($csv:String!){ adminUserBulkCreate(csv:$csv){ created errors } }`,
        { csv: bulkCsv }
      );
      setOpenBulk(false);
      setBulkCsv("");
      load();
      const errs = res.adminUserBulkCreate?.errors || [];
      toast({
        title: `作成 ${res.adminUserBulkCreate?.created || 0} 件`,
        description: errs.length ? errs.join("\n") : undefined,
      });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "一括作成に失敗しました",
        description: e?.message,
      });
    }
  }

  async function applyCustomRole(
    userId: number,
    role: string,
    enabled: boolean
  ) {
    try {
      await gql<{ adminUserSetCustomRole: boolean }>(
        `mutation($id:Int!,$role:String!,$en:Boolean!){ adminUserSetCustomRole(user_id:$id, role:$role, enabled:$en) }`,
        { id: userId, role, en: enabled }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: enabled ? "付与に失敗しました" : "削除に失敗しました",
        description: e?.message,
      });
    }
  }

  function openCustomAddDialog(u: AUser) {
    setCustomTarget(u);
    setCustomRole("");
    setOpenCustomAdd(true);
  }
  function openCustomRemoveDialog(u: AUser) {
    setCustomTarget(u);
    setCustomRole("");
    setOpenCustomRemove(true);
  }

  async function submitCustomAdd() {
    if (!customTarget) return;
    const role = customRole.trim();
    if (!role) {
      toast({ variant: "destructive", title: "ロール名を入力してください" });
      return;
    }
    await applyCustomRole(customTarget.id, role, true);
    setOpenCustomAdd(false);
  }
  async function submitCustomRemove() {
    if (!customTarget) return;
    const role = customRole.trim();
    if (!role) {
      toast({ variant: "destructive", title: "ロール名を入力してください" });
      return;
    }
    await applyCustomRole(customTarget.id, role, false);
    setOpenCustomRemove(false);
  }

  function openDeleteDialog(u: AUser) {
    setDeleteTarget(u);
    setDeleteHard(false);
    setDeleteConfirm("");
    setOpenDelete(true);
  }

  async function openNotifDialog(u: AUser) {
    setNotifTarget(u);
    try {
      const d = await gql<{
        adminUserNotificationSettings: {
          email_enabled: boolean;
          webpush_enabled: boolean;
        };
        adminUserNotificationTypePrefs: { TASK: boolean; DAYSHIFT: boolean };
        adminUserDevices: Device[];
      }>(
        `query($id:Int!){
          adminUserNotificationSettings(user_id:$id){ email_enabled webpush_enabled }
          adminUserNotificationTypePrefs(user_id:$id){ TASK DAYSHIFT }
          adminUserDevices(user_id:$id){ id user_id platform name payload is_enabled created_at updated_at }
        }`,
        { id: u.id }
      );
      setNotifEmail(d.adminUserNotificationSettings?.email_enabled ?? true);
      setNotifPush(d.adminUserNotificationSettings?.webpush_enabled ?? false);
      setTypeTask(d.adminUserNotificationTypePrefs?.TASK ?? true);
      setTypeDayShift(d.adminUserNotificationTypePrefs?.DAYSHIFT ?? true);
      setDevices(d.adminUserDevices || []);
      setOpenNotif(true);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "取得に失敗しました",
        description: e?.message,
      });
    }
  }

  async function submitNotif() {
    if (!notifTarget) return;
    try {
      // save channels (email/webpush)
      await gql<{ adminUserSetNotificationSettings: boolean }>(
        `mutation($id:Int!,$em:Boolean!,$wp:Boolean!){ adminUserSetNotificationSettings(user_id:$id, email_enabled:$em, webpush_enabled:$wp) }`,
        { id: notifTarget.id, em: notifEmail, wp: notifPush }
      );
      // save type prefs
      await gql<{ adminUserSetNotificationTypePref: boolean }>(
        `mutation($id:Int!,$tp:String!,$en:Boolean!){ adminUserSetNotificationTypePref(user_id:$id, notification_type:$tp, enabled:$en) }`,
        { id: notifTarget.id, tp: "TASK", en: typeTask }
      );
      await gql<{ adminUserSetNotificationTypePref: boolean }>(
        `mutation($id:Int!,$tp:String!,$en:Boolean!){ adminUserSetNotificationTypePref(user_id:$id, notification_type:$tp, enabled:$en) }`,
        { id: notifTarget.id, tp: "DAYSHIFT", en: typeDayShift }
      );
      setOpenNotif(false);
      toast({ title: "通知設定を更新しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }

  async function toggleDeviceEnabled(devId: number, enabled: boolean) {
    try {
      await gql<{ adminUserUpdateDevice: boolean }>(
        `mutation($id:Int!,$en:Boolean){ adminUserUpdateDevice(device_id:$id, is_enabled:$en) }`,
        { id: devId, en: enabled }
      );
      // refresh list
      if (notifTarget) {
        const r = await gql<{ adminUserDevices: Device[] }>(
          `query($id:Int!){ adminUserDevices(user_id:$id){ id user_id platform name payload is_enabled created_at updated_at } }`,
          { id: notifTarget.id }
        );
        setDevices(r.adminUserDevices || []);
      }
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }

  async function deleteDevice(devId: number) {
    if (!window.confirm("このデバイスを削除しますか？")) return;
    try {
      await gql<{ adminUserDeleteDevice: boolean }>(
        `mutation($id:Int!){ adminUserDeleteDevice(device_id:$id) }`,
        { id: devId }
      );
      if (notifTarget) {
        const r = await gql<{ adminUserDevices: Device[] }>(
          `query($id:Int!){ adminUserDevices(user_id:$id){ id user_id platform name payload is_enabled created_at updated_at } }`,
          { id: notifTarget.id }
        );
        setDevices(r.adminUserDevices || []);
      }
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }

  async function createDevice() {
    if (!notifTarget) return;
    try {
      await gql<{ adminUserCreateDevice: { id: number } }>(
        `mutation($id:Int!,$pf:String!,$name:String,$payload:String,$en:Boolean){ adminUserCreateDevice(user_id:$id, platform:$pf, name:$name, payload:$payload, enabled:$en){ id } }`,
        {
          id: notifTarget.id,
          pf: newDevPlatform,
          name: newDevName || undefined,
          payload: newDevPayload || undefined,
          en: newDevEnabled,
        }
      );
      // reset form
      setNewDevPlatform("WEBPUSH");
      setNewDevName("");
      setNewDevPayload("");
      setNewDevEnabled(true);
      // refresh list
      const r = await gql<{ adminUserDevices: Device[] }>(
        `query($id:Int!){ adminUserDevices(user_id:$id){ id user_id platform name payload is_enabled created_at updated_at } }`,
        { id: notifTarget.id }
      );
      setDevices(r.adminUserDevices || []);
      toast({ title: "デバイスを追加しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "追加に失敗しました",
        description: e?.message,
      });
    }
  }

  async function submitDelete() {
    if (!deleteTarget) return;
    if (deleteConfirm !== "削除") {
      toast({
        variant: "destructive",
        title: "確認の文字列を入力してください（削除）",
      });
      return;
    }
    try {
      await gql<{ adminUserDelete: boolean }>(
        `mutation($id:Int!,$h:Boolean){ adminUserDelete(user_id:$id, hard:$h) }`,
        { id: deleteTarget.id, h: deleteHard }
      );
      setOpenDelete(false);
      load();
      toast({ title: "削除しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">ユーザー管理</h1>
        <div className="flex gap-2">
          <Dialog open={openBulk} onOpenChange={setOpenBulk}>
            <DialogTrigger asChild>
              <Button variant="secondary">CSV一括追加</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>CSV一括追加</DialogTitle>
              </DialogHeader>
              <div className="space-y-2">
                <p className="text-sm text-muted-foreground">
                  1行につき1ユーザー。フォーマット: email,display_name,lang
                </p>
                <Textarea
                  value={bulkCsv}
                  onChange={(e) => setBulkCsv(e.target.value)}
                  rows={8}
                  placeholder={`a@example.com,山田太郎,ja\nb@example.com,John,en`}
                />
              </div>
              <DialogFooter>
                <Button onClick={submitBulk}>実行</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
          <Dialog open={openCreate} onOpenChange={setOpenCreate}>
            <DialogTrigger asChild>
              <Button>ユーザー追加</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>ユーザー追加</DialogTitle>
              </DialogHeader>
              <div className="space-y-3">
                <div>
                  <div className="text-sm mb-1">メール</div>
                  <Input
                    value={newEmail}
                    onChange={(e) => setNewEmail(e.target.value)}
                    placeholder="example@example.com"
                  />
                </div>
                <div>
                  <div className="text-sm mb-1">表示名</div>
                  <Input
                    value={newDisplayName}
                    onChange={(e) => setNewDisplayName(e.target.value)}
                    placeholder="山田 太郎"
                  />
                </div>
                <div>
                  <div className="text-sm mb-1">表示言語</div>
                  <Select value={newLang} onValueChange={setNewLang}>
                    <SelectTrigger>
                      <SelectValue placeholder="言語" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="ja">日本語</SelectItem>
                      <SelectItem value="en">English</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button onClick={submitCreate}>作成</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>
      <Card className="p-4 space-y-3">
        <div className="flex gap-2 items-end">
          <div className="flex-1 min-w-60">
            <div className="text-sm mb-1">検索（メール/表示名）</div>
            <Input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="example@example.com"
            />
          </div>
          <Button onClick={load} disabled={loading}>
            検索
          </Button>
        </div>
      </Card>
      <Card className="p-0 overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead
                className="cursor-pointer select-none"
                onClick={() => toggleSort("id")}
              >
                ID{sortLabel("id")}
              </TableHead>
              <TableHead
                className="cursor-pointer select-none"
                onClick={() => toggleSort("email")}
              >
                メール{sortLabel("email")}
              </TableHead>
              <TableHead
                className="cursor-pointer select-none"
                onClick={() => toggleSort("display_name")}
              >
                表示名{sortLabel("display_name")}
              </TableHead>
              <TableHead>ロール</TableHead>
              <TableHead>言語</TableHead>
              <TableHead>メールログイン</TableHead>
              <TableHead>ブロック</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell
                  colSpan={8}
                  className="text-center text-sm text-muted-foreground"
                >
                  読み込み中…
                </TableCell>
              </TableRow>
            ) : items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={8}
                  className="text-center text-sm text-muted-foreground"
                >
                  該当するユーザーがいません
                </TableCell>
              </TableRow>
            ) : (
              items.map((u) => (
                <TableRow key={u.id}>
                  <TableCell>{u.id}</TableCell>
                  <TableCell>{u.email}</TableCell>
                  <TableCell>{u.display_name || "-"}</TableCell>
                  <TableCell className="max-w-[240px] truncate">
                    {(u.roles || []).join(", ")}
                  </TableCell>
                  <TableCell>{u.notification_lang || "-"}</TableCell>
                  <TableCell>
                    {u.email_login_enabled ? "有効" : "無効"}
                  </TableCell>
                  <TableCell>{u.is_blocked ? "Yes" : "No"}</TableCell>
                  <TableCell className="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button size="icon" variant="ghost" aria-label="操作">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => openEditDialog(u)}>
                          編集
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => openNotifDialog(u)}>
                          通知設定
                        </DropdownMenuItem>
                        {(() => {
                          const roles = u.roles || [];
                          const hasAdmin = roles.includes("admin");
                          const hasManager = roles.includes("manager");
                          return (
                            <>
                              {hasAdmin ? (
                                <DropdownMenuItem
                                  onClick={() => removeRole(u.id, "admin")}
                                >
                                  Admin削除
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  onClick={() => addRole(u.id, "admin")}
                                >
                                  Admin付与
                                </DropdownMenuItem>
                              )}
                              {hasManager ? (
                                <DropdownMenuItem
                                  onClick={() => removeRole(u.id, "manager")}
                                >
                                  Manager削除
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  onClick={() => addRole(u.id, "manager")}
                                >
                                  Manager付与
                                </DropdownMenuItem>
                              )}
                            </>
                          );
                        })()}
                        <DropdownMenuItem
                          onClick={() => openCustomAddDialog(u)}
                        >
                          カスタムロール追加
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => openCustomRemoveDialog(u)}
                        >
                          カスタムロール削除
                        </DropdownMenuItem>
                        {u.is_blocked ? (
                          <DropdownMenuItem onClick={() => unblockUser(u.id)}>
                            ブロック解除
                          </DropdownMenuItem>
                        ) : (
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() => openBlockDialog(u)}
                          >
                            ブロック
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onClick={() => openDeleteDialog(u)}
                        >
                          削除…
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

      <Pagination className="mt-3">
        <PaginationContent>
          <PaginationItem>
            <PaginationPrevious
              href="#"
              onClick={(e) => {
                e.preventDefault();
                setPage((p) => Math.max(1, p - 1));
              }}
              className={page <= 1 ? "pointer-events-none opacity-50" : ""}
            />
          </PaginationItem>
          <PaginationItem>
            <span className="px-3 py-2 text-sm text-muted-foreground">
              {total} 件中 {items.length > 0 ? (page - 1) * pageSize + 1 : 0}–
              {Math.min(page * pageSize, total)} を表示
            </span>
          </PaginationItem>
          <PaginationItem>
            <PaginationNext
              href="#"
              onClick={(e) => {
                e.preventDefault();
                const lastPage = Math.max(1, Math.ceil(total / pageSize));
                setPage((p) => Math.min(lastPage, p + 1));
              }}
              className={
                page * pageSize >= total ? "pointer-events-none opacity-50" : ""
              }
            />
          </PaginationItem>
        </PaginationContent>
      </Pagination>

      {isMobile ? (
        <Drawer open={openEdit} onOpenChange={setOpenEdit}>
          <DrawerContent onOpenAutoFocus={(e) => e.preventDefault()}>
            <DrawerHeader className="text-left">
              <DrawerTitle>ユーザー編集</DrawerTitle>
            </DrawerHeader>
            <div className="px-4 pb-4">
              <EditUserForm />
            </div>
            <DrawerFooter>
              <Button onClick={submitEdit}>更新</Button>
            </DrawerFooter>
          </DrawerContent>
        </Drawer>
      ) : (
        <Dialog open={openEdit} onOpenChange={setOpenEdit}>
          <DialogContent onOpenAutoFocus={(e) => e.preventDefault()}>
            <DialogHeader>
              <DialogTitle>ユーザー編集</DialogTitle>
            </DialogHeader>
            <EditUserForm />
            <DialogFooter>
              <Button onClick={submitEdit}>更新</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}

      {/* Notification Settings: Responsive Drawer+Dialog */}
      {isMobile ? (
        <Drawer open={openNotif} onOpenChange={setOpenNotif}>
          <DrawerContent onOpenAutoFocus={(e) => e.preventDefault()}>
            <DrawerHeader className="text-left">
              <DrawerTitle>通知設定</DrawerTitle>
              <DrawerDescription>詳細設定とデバイス管理</DrawerDescription>
            </DrawerHeader>
            <div className="px-4 pb-4">
              <NotifSettingsContent />
            </div>
            <DrawerFooter>
              <Button onClick={submitNotif}>保存</Button>
            </DrawerFooter>
          </DrawerContent>
        </Drawer>
      ) : (
        <Dialog open={openNotif} onOpenChange={setOpenNotif}>
          <DialogContent onOpenAutoFocus={(e) => e.preventDefault()}>
            <DialogHeader>
              <DialogTitle>通知設定</DialogTitle>
            </DialogHeader>
            <NotifSettingsContent />
            <DialogFooter>
              <Button onClick={submitNotif}>保存</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}

      <Dialog open={openDelete} onOpenChange={setOpenDelete}>
        <DialogContent onOpenAutoFocus={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>ユーザー削除</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="text-sm text-muted-foreground">
              削除方法を選択してください
            </div>
            <div className="flex items-center gap-3">
              <Button
                variant={deleteHard ? "secondary" : "default"}
                onClick={() => setDeleteHard(false)}
              >
                ソフト削除 (フラグ設定)
              </Button>
              <Button
                variant={deleteHard ? "default" : "secondary"}
                onClick={() => setDeleteHard(true)}
                className="text-destructive"
              >
                物理削除
              </Button>
            </div>
            <div>
              <div className="text-sm mb-1">確認のため「削除」と入力</div>
              <Input
                value={deleteConfirm}
                onChange={(e) => setDeleteConfirm(e.target.value)}
                placeholder="削除"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="destructive"
              disabled={deleteConfirm !== "削除"}
              onClick={submitDelete}
            >
              削除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Custom Role Add Dialog */}
      <Dialog open={openCustomAdd} onOpenChange={setOpenCustomAdd}>
        <DialogContent onOpenAutoFocus={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>カスタムロールを追加</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="text-sm text-muted-foreground">
              対象: {customTarget?.email || "-"}
            </div>
            <div>
              <div className="text-sm mb-1">ロール名</div>
              <Input
                value={customRole}
                onChange={(e) => setCustomRole(e.target.value)}
                placeholder="例: reviewer"
              />
            </div>
          </div>
          <DialogFooter>
            <Button onClick={submitCustomAdd}>追加</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Custom Role Remove Dialog */}
      <Dialog open={openCustomRemove} onOpenChange={setOpenCustomRemove}>
        <DialogContent onOpenAutoFocus={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>カスタムロールを削除</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="text-sm text-muted-foreground">
              対象: {customTarget?.email || "-"}
            </div>
            <div>
              <div className="text-sm mb-1">ロール名</div>
              <Input
                value={customRole}
                onChange={(e) => setCustomRole(e.target.value)}
                placeholder="例: reviewer"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="destructive" onClick={submitCustomRemove}>
              削除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Block User Dialog */}
      <Dialog open={openBlock} onOpenChange={setOpenBlock}>
        <DialogContent onOpenAutoFocus={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>ユーザーをブロック</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="text-sm text-muted-foreground">
              対象: {blockTarget?.email || "-"}
            </div>
            <div>
              <div className="text-sm mb-1">理由（任意）</div>
              <Input
                value={blockReason}
                onChange={(e) => setBlockReason(e.target.value)}
                placeholder="例: 規約違反"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="destructive" onClick={submitBlock}>
              ブロック
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
