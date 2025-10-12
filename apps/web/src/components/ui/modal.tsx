import * as React from "react";

export function Modal({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title?: string;
  children?: React.ReactNode;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative z-10 w-[90vw] max-w-md rounded-lg border bg-card text-card-foreground shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">{title || "Notice"}</h3>
        </div>
        <div className="p-4">{children}</div>
        <div className="p-4 border-t flex justify-end">
          <button
            onClick={onClose}
            className="inline-flex items-center justify-center h-9 px-3 rounded-md border bg-background text-foreground hover:bg-accent hover:text-accent-foreground"
          >
            閉じる
          </button>
        </div>
      </div>
    </div>
  );
}
