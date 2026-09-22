import { formatYmdHm } from "@/lib/utils";
import { expiryFromLocalInput, expiryToLocalInput, InvalidTokenExpiryError } from "@/lib/token-expiry";
("use client");
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
import { Input } from "@/components/ui/input";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogDescription,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { DateTimePicker } from "@/components/ui/date-time-picker";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { CircleHelp as CircleQuestionMark } from "lucide-react";

type IItem = {
  id: number;
  description?: string | null;
  url: string;
  created_at: string;
  expires_at?: string | null;
};

export default function SettingsIcal() {
  const { dict } = useLang();
  const [items, setItems] = React.useState<IItem[]>([]);
  const [desc, setDesc] = React.useState("");
  const HELP_URL =
    "https://ritzy-lift-e38.notion.site/290a3874f8c880a6b1f6e67cef05ae56?source=copy_link";

  async function load() {
    try {
      const data = await gql<{ icalTokens: IItem[] }>(
        `query{ icalTokens{ id description url created_at expires_at } }`
      );
      setItems(data.icalTokens || []);
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
  const [expires, setExpires] = React.useState<string>(""); // local "yyyy-MM-ddTHH:mm"
  async function create() {
    try {
      await gql<{ icalCreate: { id: number; url: string } }>(
        `mutation($d:String,$e:String){ icalCreate(description:$d,expires_at:$e){ id url } }`,
        { d: desc || undefined, e: expiryFromLocalInput(expires) }
      );
      setDesc("");
      setExpires("");
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsIcal.createFailed,
        description: e instanceof InvalidTokenExpiryError ? dict.settingsDetail.invalidExpiry : e?.message,
      });
    }
  }
  async function revoke(id: number) {
    try {
      await gql<{ icalDelete: boolean }>(
        `mutation($id:Int!){ icalDelete(id:$id) }`,
        { id }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsDetail.deleteFailed,
        description: e?.message,
      });
    }
  }
  const [editId, setEditId] = React.useState<number | null>(null);
  const [editExpires, setEditExpires] = React.useState<string>(""); // local
  async function updateExpiry(id: number, value: string, original?: string | null) {
    try {
      await gql<{ icalUpdate: boolean }>(
        `mutation($id:Int!,$e:String){ icalUpdate(id:$id, expires_at:$e) }`,
        { id, e: expiryFromLocalInput(value, { update: true, original }) }
      );
      setEditId(null);
      setEditExpires("");
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsDetail.updateFailed,
        description: e instanceof InvalidTokenExpiryError ? dict.settingsDetail.invalidExpiry : e?.message,
      });
    }
  }
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteId, setDeleteId] = React.useState<number | null>(null);
  return (
    <SettingsShell>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">{dict.settingsIcal.heading}</h1>
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
          <AlertDescription>{dict.settingsIcal.shareWarning}</AlertDescription>
        </Alert>
        <Card className="p-4 space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="flex-1 min-w-60">
              <div className="text-sm mb-1">
                {dict.settingsIcal.descOptional}
              </div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder={dict.settingsIcal.exampleDescription}
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
            <Button className="w-full sm:w-auto" onClick={create}>
              {dict.settingsIcal.issueUrl}
            </Button>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{dict.settingsIcal.table.id}</TableHead>
                <TableHead>{dict.settingsIcal.table.description}</TableHead>
                <TableHead>{dict.settingsIcal.table.url}</TableHead>
                <TableHead>{dict.settingsIcal.table.createdAt}</TableHead>
                <TableHead>{dict.settingsDetail.expiry}</TableHead>
                <TableHead className="text-right">
                  {dict.settingsIcal.table.actions}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell>{it.id}</TableCell>
                  <TableCell className="max-w-[240px] truncate">
                    {it.description || "-"}
                  </TableCell>
                  <TableCell className="max-w-[360px] truncate font-mono text-xs">
                    {it.url}
                  </TableCell>
                  <TableCell>{formatYmdHm(it.created_at)}</TableCell>
                  <TableCell className="space-x-2">
                    {it.expires_at ? (
                      <>
                        <span>{formatYmdHm(it.expires_at)}</span>
                        {new Date(it.expires_at) <= new Date() && (
                          <Badge variant="destructive">{dict.settingsDetail.expired}</Badge>
                        )}
                      </>
                    ) : (
                      "-"
                    )}
                  </TableCell>
                  <TableCell className="text-right flex gap-2 justify-end">
                    <Button
                      variant="secondary"
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(it.url);
                          toast({ title: dict.common.copied });
                        } catch (e) {
                          toast({
                            variant: "destructive",
                            title: dict.settingsDetail.copyFailed,
                          });
                        }
                      }}
                    >
                      {dict.common.copy}
                    </Button>
                    <Dialog
                      onOpenChange={(open) => {
                        if (open) {
                          setEditId(it.id);
                          setEditExpires(
                            expiryToLocalInput(it.expires_at)
                          );
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
                    <Button
                      variant="destructive"
                      onClick={() => {
                        setDeleteId(it.id);
                        setDeleteOpen(true);
                      }}
                    >
                      {dict.common.delete}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
        {/* Delete confirm dialog */}
        <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{dict.settingsDetail.deleteConfirm}</DialogTitle>
              <DialogDescription>
                {dict.settingsIcal.deleteUrlConfirm}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteOpen(false)}>
                {dict.common.cancel}
              </Button>
              <Button
                variant="destructive"
                onClick={async () => {
                  if (deleteId != null) await revoke(deleteId);
                  setDeleteOpen(false);
                  setDeleteId(null);
                }}
              >
                {dict.settingsIcal.deleteUrl}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </SettingsShell>
  );
}
