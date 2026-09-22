"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogClose,
  DialogDescription,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { DateTimePicker } from "@/components/ui/date-time-picker";
import {
  Tooltip,
  TooltipProvider,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { CircleHelp as CircleQuestionMark } from "lucide-react";
import { formatYmdHm } from "@/lib/utils";
import { expiryFromLocalInput, expiryToLocalInput, InvalidTokenExpiryError } from "@/lib/token-expiry";

type ApiKey = {
  id: number;
  description?: string | null;
  is_valid: boolean;
  created_at: string;
  expires_at?: string | null;
};

export default function SettingsApiKeys() {
  const { dict } = useLang();
  const [items, setItems] = React.useState<ApiKey[]>([]);
  const [desc, setDesc] = React.useState("");
  const [expires, setExpires] = React.useState<string>(""); // local input
  const [createdToken, setCreatedToken] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [openCreate, setOpenCreate] = React.useState(false);
  const HELP_URL = "https://ritzy-lift-e38.notion.site/API-290a3874f8c880189db2e3d09128077d?source=copy_link";

  async function load() {
    try {
      const data = await gql<{ apiKeys: ApiKey[] }>(
        `query{ apiKeys{ id description is_valid created_at expires_at } }`
      );
      setItems(data.apiKeys || []);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsDetail.loadFailed,
        description: e?.message,
      });
    }
  }
  React.useEffect(() => {
    load();
  }, []);

  async function create() {
    try {
      if (creating) return;
      setCreating(true);
      const data = await gql<{ apiKeyCreate: { id: number; token: string } }>(
        `mutation($d:String,$e:String){ apiKeyCreate(description:$d, expires_at:$e){ id token } }`,
        { d: desc || undefined, e: expiryFromLocalInput(expires) }
      );
      setCreatedToken(data.apiKeyCreate.token);
      setDesc("");
      setExpires("");
      await load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsApiKeys.issueFailed,
        description: e instanceof InvalidTokenExpiryError ? dict.settingsDetail.invalidExpiry : e?.message,
      });
    } finally {
      setCreating(false);
    }
  }
  async function toggleValid(id: number, isValid: boolean) {
    try {
      await gql<{ apiKeyUpdate: { id: number; is_valid: boolean } }>(
        `mutation($id:Int!,$v:Boolean){ apiKeyUpdate(id:$id, is_valid:$v){ id is_valid } }`,
        { id, v: !isValid }
      );
      load();
    } catch (e: any) {
      toast({
        title: dict.settingsDetail.updateFailed,
        description: e?.message,
      });
    }
  }

  async function updateExpiry(id: number, value: string, original?: string | null) {
    try {
      await gql<{ apiKeyUpdate: { id: number; is_valid: boolean } }>(
        `mutation($id:Int!,$e:String){ apiKeyUpdate(id:$id, expires_at:$e){ id is_valid } }`,
        { id, e: expiryFromLocalInput(value, { update: true, original }) }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsDetail.updateFailed,
        description: e instanceof InvalidTokenExpiryError ? dict.settingsDetail.invalidExpiry : e?.message,
      });
    }
  }

  const [editId, setEditId] = React.useState<number | null>(null);
  const [editExpires, setEditExpires] = React.useState<string>("");

  // Keyboard shortcut: Cmd/Ctrl+N to open and issue new key
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const key = (e.key || "").toLowerCase();
      if ((e.metaKey || e.ctrlKey) && key === "n") {
        e.preventDefault();
        setOpenCreate(true);
      }
    };
    window.addEventListener("keydown", onKey as any);
    return () => window.removeEventListener("keydown", onKey as any);
  }, []);

  return (
    <SettingsShell>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">
            {dict.settingsApiKeys.heading}
          </h1>
          {HELP_URL && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <a href={HELP_URL} target="_blank" rel="noreferrer">
                    <Button variant="ghost" size="icon" aria-label={dict.settingsDetail.help}>
                      <CircleQuestionMark />
                    </Button>
                  </a>
                </TooltipTrigger>
                <TooltipContent>{dict.settingsDetail.help}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
        <Alert>
          <AlertTitle>{dict.common.warning}</AlertTitle>
          <AlertDescription>
            {dict.settingsApiKeys.shareWarning}
          </AlertDescription>
        </Alert>
        <Card className="p-4 space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="min-w-60 flex-1">
              <div className="text-sm mb-1">
                {dict.settingsApiKeys.descOptional}
              </div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder={dict.settingsApiKeys.exampleDescription}
              />
            </div>
            <div className="min-w-60">
              <div className="text-sm mb-1">{dict.settingsDetail.expiryOptional}</div>
              <DateTimePicker
                value={expires}
                onChange={setExpires}
                placeholder={dict.settingsDetail.selectDateTime}
              />
            </div>
            <Dialog
              open={openCreate}
              onOpenChange={async (open) => {
                setOpenCreate(open);
                if (open) {
                  setCreatedToken(null);
                  setCreating(false);
                  await create();
                } else {
                  setCreating(false);
                }
              }}
            >
              <DialogTrigger asChild>
                <Button type="button" onClick={() => setOpenCreate(true)}>
                  {dict.settingsApiKeys.issueNew}
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{dict.settingsApiKeys.dialogTitle}</DialogTitle>
                </DialogHeader>
                {createdToken ? (
                  <div className="space-y-2">
                    <div className="text-sm">
                      {dict.settingsApiKeys.tokenWarn}
                    </div>
                    <code className="block break-all rounded border bg-muted/30 p-2 text-sm">
                      {createdToken}
                    </code>
                    <div className="flex gap-2">
                      <Button
                        onClick={() =>
                          navigator.clipboard.writeText(createdToken!)
                        }
                      >
                        {dict.common.copy}
                      </Button>
                      <DialogClose asChild>
                        <Button
                          variant="secondary"
                          onClick={() => setCreatedToken(null)}
                        >
                          {dict.common.close}
                        </Button>
                      </DialogClose>
                    </div>
                  </div>
                ) : (
                  <DialogFooter>
                    <Button disabled>
                      {creating
                        ? dict.common.loading
                        : dict.settingsApiKeys.issueNew}
                    </Button>
                  </DialogFooter>
                )}
              </DialogContent>
            </Dialog>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{dict.settingsApiKeys.table.id}</TableHead>
                <TableHead>{dict.settingsApiKeys.table.description}</TableHead>
                <TableHead>{dict.settingsApiKeys.table.status}</TableHead>
                <TableHead>{dict.settingsApiKeys.table.createdAt}</TableHead>
                <TableHead>{dict.settingsDetail.expiry}</TableHead>
                <TableHead className="text-right">
                  {dict.settingsApiKeys.table.actions}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell>{it.id}</TableCell>
                  <TableCell className="max-w-[300px] truncate">
                    {it.description || "-"}
                  </TableCell>
                  <TableCell>
                    {it.expires_at && new Date(it.expires_at) <= new Date() ? (
                      <Badge variant="destructive">{dict.settingsDetail.expired}</Badge>
                    ) : it.is_valid ? (
                      dict.common.enabled
                    ) : (
                      dict.common.disabled
                    )}
                  </TableCell>
                  <TableCell>{formatYmdHm(it.created_at)}</TableCell>
                  <TableCell>
                    {it.expires_at ? formatYmdHm(it.expires_at) : "-"}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex gap-2 justify-end">
                      <Button
                        variant="secondary"
                        onClick={() => toggleValid(it.id, it.is_valid)}
                      >
                        {it.is_valid ? dict.common.disable : dict.common.enable}
                      </Button>
                      <Dialog
                        onOpenChange={(open) => {
                          if (open) {
                            setEditId(it.id);
                            setEditExpires(expiryToLocalInput(it.expires_at));
                          } else {
                            setEditId(null);
                            setEditExpires("");
                          }
                        }}
                      >
                        <DialogTrigger asChild>
                          <Button variant="outline">{dict.settingsDetail.updateExpiry}</Button>
                        </DialogTrigger>
                        <DialogContent>
                          <DialogHeader>
                            <DialogTitle>{dict.settingsDetail.expiryDialog}</DialogTitle>
                            <DialogDescription>
                              {dict.settingsDetail.clearExpiryHint}
                            </DialogDescription>
                          </DialogHeader>
                          <DateTimePicker
                            value={editExpires}
                            onChange={setEditExpires}
                            placeholder={dict.settingsDetail.selectDateTime}
                          />
                          <DialogFooter>
                            <Button
                              onClick={() => updateExpiry(editId!, editExpires, it.expires_at)}
                            >
                              {dict.common.save}
                            </Button>
                          </DialogFooter>
                        </DialogContent>
                      </Dialog>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      </div>
    </SettingsShell>
  );
}
