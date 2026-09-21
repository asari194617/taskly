import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { useLang } from "@/react/LanguageProvider";

export function TaskReportDialog({
  open,
  setOpen,
  targetId,
  reason,
  setReason,
  comment,
  setComment,
  submittingId,
  onSubmit,
}: {
  open: boolean;
  setOpen: (value: boolean) => void;
  targetId: number | null;
  reason: string;
  setReason: (value: string) => void;
  comment: string;
  setComment: (value: string) => void;
  submittingId: number | null;
  onSubmit: (id: number) => void;
}) {
  const { dict } = useLang();
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{dict.tasks.reportSubmit}</DialogTitle>
          <DialogDescription>{dict.tasks.enterRequiredInfo}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="report-reason">
              {dict.tasks.reportReasonLabel ?? "Reason"}
            </Label>
            <Input
              id="report-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder={
                dict.tasks.reportReasonPlaceholder ?? "Spam/Inappropriate/etc."
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="report-comment">
              {dict.tasks.reportCommentLabel ?? "Comment"}
            </Label>
            <Input
              id="report-comment"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder={
                dict.tasks.reportCommentPlaceholder ?? "Optional details"
              }
            />
          </div>
          <DialogFooter>
            <Button
              onClick={() => {
                if (targetId != null) {
                  onSubmit(targetId);
                }
              }}
              disabled={targetId != null && submittingId === targetId}
            >
              {dict.tasks.reportSubmit}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// Delete confirm dialog mounted at root of TasksList
// Note: placed after component to keep JSX readable
export function TasksListDeleteDialog({
  open,
  setOpen,
  onConfirm,
}: {
  open: boolean;
  setOpen: (v: boolean) => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>削除の確認</DialogTitle>
          <DialogDescription>
            この課題を削除します。よろしいですか？
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            キャンセル
          </Button>
          <Button
            variant="destructive"
            onClick={() => {
              onConfirm();
              setOpen(false);
            }}
          >
            削除する
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
