import { useEffect, useState } from "react";
import {
  Copy,
  Download,
  ExternalLink,
  LoaderCircle,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useLang } from "@/react/LanguageProvider";

async function fetchImage(url: string): Promise<Blob> {
  const response = await fetch(url, {
    credentials: "omit",
    referrerPolicy: "no-referrer",
    cache: "no-store",
  });
  if (!response.ok) throw new Error("Image request failed");
  return response.blob();
}

async function clipboardPNG(url: string): Promise<Blob> {
  const blob = await fetchImage(url);
  if (blob.type === "image/png") return blob;
  const objectURL = URL.createObjectURL(blob);
  try {
    const image = new Image();
    image.src = objectURL;
    await image.decode();
    const canvas = document.createElement("canvas");
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Canvas unavailable");
    context.drawImage(image, 0, 0);
    return await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (png) =>
          png ? resolve(png) : reject(new Error("PNG conversion failed")),
        "image/png",
      ),
    );
  } finally {
    URL.revokeObjectURL(objectURL);
  }
}

function PreviewContent({
  url,
  alt,
  filename,
}: {
  url: string;
  alt: string;
  filename: string;
}) {
  const { dict } = useLang();
  const labels = dict.imagePreview;
  const [stage, setStage] = useState<HTMLDivElement | null>(null);
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [natural, setNatural] = useState({ width: 0, height: 0 });
  const [zoom, setZoom] = useState(1);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const canCopy =
    typeof ClipboardItem !== "undefined" && !!navigator.clipboard?.write;

  useEffect(() => {
    const element = stage;
    if (!element) return;
    const observer = new ResizeObserver(([entry]) => {
      setViewport({
        width: entry.contentRect.width,
        height: entry.contentRect.height,
      });
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [stage]);

  const scale =
    natural.width && viewport.width
      ? Math.min(
          viewport.width / natural.width,
          viewport.height / natural.height,
          1,
        ) * zoom
      : 0;

  async function download() {
    setBusy(true);
    setMessage("");
    try {
      const blob = await fetchImage(url);
      const extension =
        (
          {
            "image/png": "png",
            "image/jpeg": "jpg",
            "image/webp": "webp",
            "image/gif": "gif",
          } as Record<string, string>
        )[blob.type] || "img";
      const objectURL = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = objectURL;
      link.download = `${filename}.${extension}`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      // Give the browser time to start reading the download before releasing it.
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 60_000);
    } catch {
      setMessage(labels.downloadFailed);
    } finally {
      setBusy(false);
    }
  }

  async function copy() {
    setBusy(true);
    setMessage("");
    try {
      // Start clipboard.write during the click to preserve Safari's user activation.
      const png = clipboardPNG(url);
      // A permission rejection can happen before the clipboard consumes this promise.
      void png.catch(() => {});
      await navigator.clipboard.write([
        new ClipboardItem({ "image/png": png }),
      ]);
      setMessage(dict.common.copied);
    } catch {
      setMessage(labels.copyFailed);
    } finally {
      setBusy(false);
    }
  }

  return (
    <DialogContent
      showCloseButton={false}
      className="flex h-[90dvh] w-[calc(100%-2rem)] max-w-none flex-col gap-3 overflow-hidden p-3 sm:max-w-6xl sm:p-4"
    >
      <div className="flex items-center justify-between gap-2">
        <DialogTitle className="min-w-0 truncate leading-normal">
          {alt}
        </DialogTitle>
        <DialogClose asChild>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            aria-label={dict.common.close}
            title={dict.common.close}
          >
            <X />
          </Button>
        </DialogClose>
      </div>
      <DialogDescription className="sr-only">
        {labels.description}
      </DialogDescription>
      <div
        className="flex flex-wrap items-center gap-1"
        role="group"
        aria-label={labels.controls}
      >
        <Button
          type="button"
          size="icon"
          variant="outline"
          disabled={zoom <= 0.25}
          aria-label={labels.zoomOut}
          title={labels.zoomOut}
          onClick={() => setZoom((value) => Math.max(0.25, value - 0.25))}
        >
          <ZoomOut />
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="min-w-16 tabular-nums"
          title={labels.resetZoom}
          aria-label={labels.resetZoom}
          onClick={() => setZoom(1)}
        >
          {Math.round(zoom * 100)}%
        </Button>
        <Button
          type="button"
          size="icon"
          variant="outline"
          disabled={zoom >= 4}
          aria-label={labels.zoomIn}
          title={labels.zoomIn}
          onClick={() => setZoom((value) => Math.min(4, value + 0.25))}
        >
          <ZoomIn />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="outline"
          disabled={busy || failed}
          aria-label={labels.download}
          title={labels.download}
          onClick={download}
        >
          <Download />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="outline"
          disabled={busy || failed || !canCopy}
          aria-label={labels.copy}
          title={canCopy ? labels.copy : labels.copyUnsupported}
          onClick={copy}
        >
          <Copy />
        </Button>
        <Button asChild size="icon" variant="outline">
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={labels.openNewTab}
            title={labels.openNewTab}
          >
            <ExternalLink />
          </a>
        </Button>
        {busy && (
          <LoaderCircle
            className="size-4 animate-spin"
            aria-label={dict.common.loading}
          />
        )}
      </div>
      <div
        ref={setStage}
        className="min-h-0 flex-1 overflow-auto rounded-md bg-muted/50"
        tabIndex={0}
        role="region"
        aria-label={labels.preview}
      >
        {failed ? (
          <p role="alert" className="p-4 text-center">
            {labels.loadFailed}
          </p>
        ) : (
          <div
            className="grid min-h-full min-w-full place-items-center"
            style={{
              width: natural.width * scale || "100%",
              height: natural.height * scale || "100%",
            }}
          >
            <img
              src={url}
              alt={alt}
              referrerPolicy="no-referrer"
              draggable={false}
              className="max-w-none"
              style={{
                width: natural.width * scale || undefined,
                height: natural.height * scale || undefined,
                visibility: scale ? "visible" : "hidden",
              }}
              onLoad={(event) =>
                setNatural({
                  width: event.currentTarget.naturalWidth,
                  height: event.currentTarget.naturalHeight,
                })
              }
              onError={() => setFailed(true)}
            />
          </div>
        )}
      </div>
      <p role="status" className="min-h-5 text-sm text-muted-foreground">
        {message || (!canCopy ? labels.copyUnsupported : "")}
      </p>
    </DialogContent>
  );
}

export function ImagePreview({
  url,
  alt,
  filename,
}: {
  url: string;
  alt: string;
  filename: string;
}) {
  const { dict } = useLang();
  const [open, setOpen] = useState(false);
  return (
    <div className="min-w-0" onClick={(event) => event.stopPropagation()}>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild>
          <button
            type="button"
            aria-label={`${dict.imagePreview.preview}: ${alt}`}
            className="block w-full cursor-zoom-in rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <img
              src={url}
              alt={alt}
              loading="lazy"
              decoding="async"
              referrerPolicy="no-referrer"
              className="h-40 w-full rounded-md border object-cover"
            />
          </button>
        </DialogTrigger>
        {open && <PreviewContent url={url} alt={alt} filename={filename} />}
      </Dialog>
    </div>
  );
}
