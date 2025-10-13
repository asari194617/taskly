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
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";

type Cycle = {
  id: number;
  begin_date: string;
  finish_date: string;
  description: string;
};
type Pair = {
  id: number;
  cycle_id: number;
  user1_id: number;
  user2_id: number;
  position: number;
};

export default function AdminDayShift() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [cycles, setCycles] = React.useState<Cycle[]>([]);
  const [cid, setCid] = React.useState<string>("");
  const [pairs, setPairs] = React.useState<Pair[]>([]);
  const [begin, setBegin] = React.useState("");
  const [finish, setFinish] = React.useState("");
  const [desc, setDesc] = React.useState("");
  const [u1, setU1] = React.useState("");
  const [u2, setU2] = React.useState("");
  const [loadingCycles, setLoadingCycles] = React.useState(false);
  const [loadingPairs, setLoadingPairs] = React.useState(false);

  async function loadCycles() {
    try {
      setLoadingCycles(true);
      const d = await gql<{ adminDayshiftCycles: Cycle[] }>(
        `query{ adminDayshiftCycles{ id begin_date finish_date description } }`
      );
      setCycles(d.adminDayshiftCycles || []);
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
  async function loadPairs() {
    if (!cid) {
      setPairs([]);
      return;
    }
    try {
      setLoadingPairs(true);
      const d = await gql<{ adminDayshiftPairs: Pair[] }>(
        `query($cid:Int!){ adminDayshiftPairs(cycle_id:$cid){ id cycle_id user1_id user2_id position } }`,
        { cid: Number(cid) }
      );
      setPairs(d.adminDayshiftPairs || []);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoadingPairs(false);
    }
  }
  React.useEffect(() => {
    if (token) loadCycles();
  }, [token]);
  React.useEffect(() => {
    loadPairs();
  }, [cid]);

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
  async function createPair() {
    if (!cid || !u1 || !u2) return;
    try {
      await gql<{ adminDayshiftCreatePair: { id: number } }>(
        `mutation($cid:Int!,$u1:Int!,$u2:Int!){ adminDayshiftCreatePair(cycle_id:$cid, user1_id:$u1, user2_id:$u2){ id } }`,
        { cid: Number(cid), u1: Number(u1), u2: Number(u2) }
      );
      setU1("");
      setU2("");
      loadPairs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }
  async function deletePair(id: number) {
    if (!cid) return;
    try {
      await gql<{ adminDayshiftDeletePair: boolean }>(
        `mutation($cid:Int!,$pid:Int!){ adminDayshiftDeletePair(cycle_id:$cid, pair_id:$pid) }`,
        { cid: Number(cid), pid: id }
      );
      loadPairs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }

  return (
    <div className="space-y-8">
      <div className="space-y-3">
        <h1 className="text-xl font-semibold">日直管理</h1>
        <Card className="p-4 space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <div className="text-sm mb-1">開始日</div>
              <Input
                type="date"
                value={begin}
                onChange={(e) => setBegin(e.target.value)}
              />
            </div>
            <div>
              <div className="text-sm mb-1">終了日</div>
              <Input
                type="date"
                value={finish}
                onChange={(e) => setFinish(e.target.value)}
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
                cycles.map((c) => (
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
        </Card>
      </div>
      <div className="space-y-3">
        <h2 className="text-lg font-medium">ペア</h2>
        <Card className="p-4 space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <div className="text-sm mb-1">Cycle ID</div>
              <Input
                type="number"
                value={cid}
                onChange={(e) => setCid(e.target.value)}
                placeholder="選択または入力"
              />
            </div>
            <div>
              <div className="text-sm mb-1">User1 ID</div>
              <Input
                type="number"
                value={u1}
                onChange={(e) => setU1(e.target.value)}
              />
            </div>
            <div>
              <div className="text-sm mb-1">User2 ID</div>
              <Input
                type="number"
                value={u2}
                onChange={(e) => setU2(e.target.value)}
              />
            </div>
          </div>
          <div className="flex justify-end">
            <Button onClick={createPair} disabled={!cid || !u1 || !u2}>
              ペア追加
            </Button>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Cycle</TableHead>
                <TableHead>User1</TableHead>
                <TableHead>User2</TableHead>
                <TableHead>Position</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingPairs ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-sm text-muted-foreground"
                  >
                    読み込み中…
                  </TableCell>
                </TableRow>
              ) : pairs.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-sm text-muted-foreground"
                  >
                    ペアがありません
                  </TableCell>
                </TableRow>
              ) : (
                pairs.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell>{p.id}</TableCell>
                    <TableCell>{p.cycle_id}</TableCell>
                    <TableCell>{p.user1_id}</TableCell>
                    <TableCell>{p.user2_id}</TableCell>
                    <TableCell>{p.position}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="destructive"
                        onClick={() => deletePair(p.id)}
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
      </div>
    </div>
  );
}
