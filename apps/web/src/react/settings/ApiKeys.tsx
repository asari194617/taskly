"use client"
import * as React from "react"
import SettingsShell from "@/react/SettingsShell"
import { Card } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"

type ApiKey = { id: number; description?: string | null; is_valid: boolean; created_at: string }

export default function SettingsApiKeys() {
  const token = (typeof window !== "undefined" && localStorage.getItem("api_token")) || ""
  const [items, setItems] = React.useState<ApiKey[]>([])
  const [desc, setDesc] = React.useState("")
  const [createdToken, setCreatedToken] = React.useState<string | null>(null)

  async function load() {
    const res = await fetch("/apikeys", { headers: { Authorization: token } })
    if (res.ok) setItems(await res.json())
  }
  React.useEffect(() => { if (token) load() }, [token])

  async function create() {
    const res = await fetch("/apikeys", { method: "POST", headers: { "Content-Type": "application/json", Authorization: token }, body: JSON.stringify({ description: desc || undefined }) })
    if (res.ok) {
      const data = await res.json();
      setCreatedToken(data.token)
      setDesc("")
      await load()
    }
  }
  async function toggleValid(id: number, isValid: boolean) {
    const res = await fetch(`/apikeys/${id}`, { method: "PATCH", headers: { "Content-Type": "application/json", Authorization: token }, body: JSON.stringify({ is_valid: !isValid }) })
    if (res.ok) load()
  }

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">APIキー</h1>
        <Card className="p-4 space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="min-w-60 flex-1">
              <div className="text-sm mb-1">説明（任意）</div>
              <Input value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="例: CI/CD" />
            </div>
            <Dialog>
              <DialogTrigger asChild>
                <Button onClick={(e) => { e.preventDefault(); }}>新しいAPIキーを発行</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>APIキーを発行</DialogTitle>
                </DialogHeader>
                {createdToken ? (
                  <div className="space-y-2">
                    <div className="text-sm">このトークンは一度しか表示されません。必ず保存してください。</div>
                    <code className="block break-all rounded border bg-muted/30 p-2 text-sm">{createdToken}</code>
                    <div className="flex gap-2">
                      <Button onClick={() => navigator.clipboard.writeText(createdToken!)}>コピー</Button>
                      <Button variant="secondary" onClick={() => setCreatedToken(null)}>閉じる</Button>
                    </div>
                  </div>
                ) : (
                  <DialogFooter>
                    <Button onClick={create}>発行する</Button>
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
                <TableHead>ID</TableHead>
                <TableHead>説明</TableHead>
                <TableHead>状態</TableHead>
                <TableHead>作成日</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell>{it.id}</TableCell>
                  <TableCell className="max-w-[300px] truncate">{it.description || "-"}</TableCell>
                  <TableCell>{it.is_valid ? "有効" : "無効"}</TableCell>
                  <TableCell>{new Date(it.created_at).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="secondary" onClick={() => toggleValid(it.id, it.is_valid)}>{it.is_valid ? "無効化" : "有効化"}</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      </div>
    </SettingsShell>
  )
}
