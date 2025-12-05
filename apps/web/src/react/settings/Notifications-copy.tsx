"use client";
import { AlertCircleIcon } from "lucide-react";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { useLang } from "@/react/LanguageProvider";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:3063";

export default function SettingsNotifications() {
  const { dict } = useLang();
  const [isOpenWebPushModal, setIsOpenWebPushModal] = React.useState(false);
  const perm = "denied";

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">
          {dict.settingsNotifications.heading}
        </h1>
        <Accordion type="multiple" className="w-full">
          <AccordionItem value="item-1">
            <AccordionTrigger>通知方法</AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <Card className="p-4 space-y-3">
                <div className="font-bold flex items-center justify-between">
                  <div>メール</div>
                  <Switch />
                </div>
              </Card>
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    Webプッシュ通知
                  </p>
                  <Switch />
                </div>
                <div className="flex items-center justify-end">
                  <Dialog
                    open={isOpenWebPushModal}
                    onOpenChange={setIsOpenWebPushModal}
                  >
                    <DialogTrigger asChild>
                      {/* Webプッシュ通知自体が無効の場合はdisabledにする */}
                      <Button variant="outline">通知端末の設定</Button>
                    </DialogTrigger>
                    <DialogContent className="sm:max-w-md">
                      <DialogHeader>
                        <DialogTitle>通知端末の設定</DialogTitle>
                        <DialogDescription>
                          Web
                          Push通知設定をしている各端末で、通知を受け取るかどうかを設定できます。
                        </DialogDescription>
                      </DialogHeader>
                      <div className="flex flex-col gap-4">
                        <Card className="p-4 space-y-3">
                          <div className="flex items-center justify-between">
                            <small className="text-sm leading-none font-medium">
                              iPhone
                            </small>
                            <Switch />
                          </div>
                        </Card>
                        <Card className="p-4 space-y-3">
                          <div className="flex items-center justify-between">
                            <small className="text-sm leading-none font-medium">
                              iPad
                            </small>
                            <Switch />
                          </div>
                        </Card>
                        <Card className="p-4 space-y-3">
                          <div className="flex items-center justify-between">
                            <small className="text-sm leading-none font-medium">
                              Mac
                            </small>
                            <Switch />
                          </div>
                        </Card>
                      </div>
                      <DialogFooter className="sm:justify-start">
                        <DialogClose asChild>
                          <Button type="button" variant="secondary">
                            閉じる
                          </Button>
                        </DialogClose>
                      </DialogFooter>
                    </DialogContent>
                  </Dialog>
                </div>
              </Card>
            </AccordionContent>
          </AccordionItem>
          <AccordionItem value="item-2">
            <AccordionTrigger>通知の種類</AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <Card className="p-4 space-y-3">
                {/* 
                  # 期限が近い課題
                  期限が近い課題の通知自体が無効な場合は、朝夕の通知はdisabledにする
                */}
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    期限が近い課題
                  </p>
                  <Switch />
                </div>
                <div className="flex items-center justify-between">
                  <p className="ml-1 leading-7 [&:not(:first-child)]:mt-6">
                    朝の通知
                  </p>
                  <Switch />
                </div>
                <div className="flex items-center justify-between">
                  <p className="ml-1 leading-7 [&:not(:first-child)]:mt-6">
                    夕方の通知
                  </p>
                  <Switch />
                </div>
              </Card>
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    日直のお知らせ
                  </p>
                  <Switch />
                </div>
              </Card>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
        <Alert>
          <AlertCircleIcon />
          <AlertTitle>通知を受け取るには、通知方法と通知の種類の両方が有効である必要があります。</AlertTitle>
          <AlertDescription>
            いずれかが無効の場合、配信されません。また、Webプッシュ通知を有効にするには、ブラウザの通知許可設定で「許可」にする必要があります。
          </AlertDescription>
        </Alert>
        <Accordion type="multiple" className="w-full">
          <AccordionItem value="item-2">
            <AccordionTrigger>その他の設定</AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    テスト通知
                  </p>
                  <Button variant="outline">テスト通知を送信</Button>
                </div>
              </Card>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </div>
    </SettingsShell>
  );
}
