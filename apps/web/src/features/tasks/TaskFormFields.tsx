import * as React from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DeadlinePicker, type DeadlineMeta } from "@/components/DeadlinePicker";
import { SubjectSelect } from "./SubjectSelect";
import {
  TASK_IMAGE_MAX_BYTES,
  TASK_IMAGE_MAX_COUNT,
  TASK_IMAGE_TYPES,
  type Subject,
  type TaskImageUpload,
} from "./api";
import { useLang } from "@/react/LanguageProvider";
import { taskTypeOptions } from "./completion-defaults";
import {
  Attachment,
  AttachmentAction,
  AttachmentActions,
  AttachmentContent,
  AttachmentDescription,
  AttachmentMedia,
  AttachmentTitle,
} from "@/components/ui/attachment";
import { ImagePlus, X } from "lucide-react";

export type TaskFormImage = {
  id: string;
  file?: File;
  key?: string;
  url?: string;
  upload?: TaskImageUpload;
};

export type TaskFormValues = {
  title: string;
  subjectId: string;
  date: string;
  meta: DeadlineMeta | null;
  requireHandIn: boolean;
  selfCheck: boolean;
  completionDisabled: boolean;
  type: string;
  description: string;
  images: TaskFormImage[];
  submitting?: boolean;
};

export type TaskFormSetters = {
  title: (value: string) => void;
  subjectId: (value: string) => void;
  date: (value: string) => void;
  meta: (value: DeadlineMeta | null) => void;
  requireHandIn: (value: boolean) => void;
  selfCheck: (value: boolean) => void;
  completionDisabled: (value: boolean) => void;
  type: (value: string) => void;
  description: (value: string) => void;
  images: (value: TaskFormImage[]) => void;
};

function FileImagePreview({ file, name }: { file: File; name: string }) {
  const [url, setURL] = React.useState("");
  React.useEffect(() => {
    const nextURL = URL.createObjectURL(file);
    setURL(nextURL);
    return () => URL.revokeObjectURL(nextURL);
  }, [file]);
  return url ? (
    <img src={url} alt={name} className="h-full w-full object-cover" />
  ) : null;
}

export function TaskFormFields({
  id,
  values,
  set,
  subjects,
  mode = "create",
  compact = false,
}: {
  id: string;
  values: TaskFormValues;
  set: TaskFormSetters;
  subjects: Subject[];
  mode?: "create" | "edit";
  compact?: boolean;
}) {
  const { lang, dict } = useLang();
  const [imageError, setImageError] = React.useState("");
  const [isDraggingImage, setIsDraggingImage] = React.useState(false);
  const imageDragDepth = React.useRef(0);
  const canAddImages =
    !values.submitting && values.images.length < TASK_IMAGE_MAX_COUNT;
  const imageDropActive = canAddImages && isDraggingImage;
  const resetImageDrag = () => {
    imageDragDepth.current = 0;
    setIsDraggingImage(false);
  };
  React.useEffect(() => {
    if (!canAddImages) resetImageDrag();
  }, [canAddImages]);
  const appendFiles = (files: FileList | File[]) => {
    if (values.submitting) return;
    const available = TASK_IMAGE_MAX_COUNT - values.images.length;
    const selected = Array.from(files);
    if (available <= 0 || selected.length > available) {
      setImageError(dict.tasks.imageTooMany);
    } else {
      setImageError("");
    }
    const valid = selected.filter((file) => {
      const accepted =
        TASK_IMAGE_TYPES.includes(file.type) &&
        file.size > 0 &&
        file.size <= TASK_IMAGE_MAX_BYTES;
      if (!accepted) setImageError(dict.tasks.imageInvalid);
      return accepted;
    });
    const additions = valid.slice(0, Math.max(available, 0)).map((file) => ({
      id: `${file.name}-${file.size}-${file.lastModified}-${crypto.randomUUID()}`,
      file,
    }));
    set.images([...values.images, ...additions]);
  };
  return (
    <fieldset
      disabled={values.submitting}
      className={`min-w-0 grid ${compact ? "grid-cols-1" : "grid-cols-2"} gap-4 [&>div]:min-w-0`}
    >
      <div className="col-span-full">
        <Label htmlFor={`${id}-title`}>{dict.tasks.title}</Label>
        <Input
          className={compact ? "h-11 text-base" : undefined}
          required
          id={`${id}-title`}
          value={values.title}
          onChange={(event) => set.title(event.target.value)}
          placeholder={dict.tasks.titlePlaceholder}
        />
      </div>
      <div>
        <Label htmlFor={`${id}-subject`}>{dict.tasks.subject}</Label>
        <SubjectSelect
          id={`${id}-subject`}
          value={values.subjectId}
          onChange={set.subjectId}
          subjects={subjects}
          language={lang}
          searchLabel={dict.common.search}
          emptyLabel={dict.tasks.notFound}
          placeholder={dict.tasks.selectPlaceholder}
        />
      </div>
      <div>
        <Label htmlFor={`${id}-date`}>{dict.tasks.deadline}</Label>
        <DeadlinePicker
          id={`${id}-date`}
          inline={compact}
          date={values.date}
          meta={values.meta}
          onChange={(date, meta) => {
            set.date(date);
            set.meta(meta);
          }}
        />
      </div>
      <details className="col-span-full" open={compact ? undefined : true}>
        <summary
          className={
            compact ? "cursor-pointer py-3 text-base font-medium" : "hidden"
          }
        >
          {lang === "ja" ? "詳細を追加（任意）" : "Add details (optional)"}
        </summary>
        <div className="grid grid-cols-1 gap-4 pt-2">
          <div
            className={`col-span-full flex items-center gap-2 ${mode === "edit" ? "order-last" : ""}`}
          >
            <Switch
              id={`${id}-req`}
              checked={values.requireHandIn}
              onCheckedChange={set.requireHandIn}
            />
            <Label htmlFor={`${id}-req`}>{dict.tasks.requireHandIn}</Label>
          </div>
          <div
            className={`col-span-full flex items-center gap-2 -mt-2 ${mode === "edit" ? "order-last" : ""}`}
          >
            <Switch
              id={`${id}-scr`}
              checked={values.selfCheck}
              onCheckedChange={set.selfCheck}
            />
            <Label htmlFor={`${id}-scr`}>{dict.tasks.selfCheckRequired}</Label>
          </div>
          <div className={mode === "edit" ? "col-span-full" : undefined}>
            <Label htmlFor={`${id}-type`}>{dict.tasks.type}</Label>
            <Select value={values.type} onValueChange={set.type}>
              <SelectTrigger
                id={`${id}-type`}
                className={compact ? "min-h-11 w-full text-base" : undefined}
              >
                <SelectValue placeholder={dict.tasks.selectPlaceholder} />
              </SelectTrigger>
              <SelectContent>
                {taskTypeOptions.map(({ type, label }) => (
                  <SelectItem key={type} value={type}>
                    {dict.tasks[label]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="col-span-full flex items-center gap-2">
            <Switch
              id={`${id}-completion`}
              checked={values.completionDisabled}
              onCheckedChange={set.completionDisabled}
            />
            <Label htmlFor={`${id}-completion`}>
              {dict.tasks.disableCompletion}
            </Label>
          </div>
          <div className="col-span-full">
            <Label htmlFor={`${id}-desc`}>
              {dict.tasks.description} {dict.tasks.optional}
            </Label>
            <textarea
              id={`${id}-desc`}
              className="w-full rounded-md border bg-background px-3 py-2 text-base md:text-sm"
              rows={3}
              value={values.description}
              onChange={(event) => set.description(event.target.value)}
            />
          </div>
          <div className="col-span-full">
            <Label htmlFor={`${id}-images`}>
              {dict.tasks.images} {dict.tasks.optional}
            </Label>
            <label
              htmlFor={`${id}-images`}
              data-testid={`${id}-image-dropzone`}
              className={`mt-1 flex min-h-24 cursor-pointer flex-col items-center justify-center rounded-md border border-dashed px-4 py-3 text-center transition-colors ${imageDropActive ? "border-primary bg-primary/10 ring-2 ring-primary/30" : "bg-muted/30 hover:bg-muted/50"}`}
              onDragEnter={(event) => {
                event.preventDefault();
                if (!canAddImages || !event.dataTransfer.types.includes("Files"))
                  return;
                imageDragDepth.current += 1;
                setIsDraggingImage(true);
              }}
              onDragOver={(event) => {
                event.preventDefault();
                event.dataTransfer.dropEffect = canAddImages ? "copy" : "none";
              }}
              onDragLeave={(event) => {
                event.preventDefault();
                imageDragDepth.current = Math.max(0, imageDragDepth.current - 1);
                if (imageDragDepth.current === 0) setIsDraggingImage(false);
              }}
              onDrop={(event) => {
                event.preventDefault();
                resetImageDrag();
                appendFiles(event.dataTransfer.files);
              }}
            >
              <ImagePlus
                className={`mb-1 h-5 w-5 ${imageDropActive ? "text-primary" : "text-muted-foreground"}`}
              />
              <span className="text-sm">
                {compact
                  ? lang === "ja"
                    ? "通知や課題の画像を選択"
                    : "Choose a task or notification image"
                  : dict.tasks.imageDrop}
              </span>
              <span className="text-xs text-muted-foreground">
                {dict.tasks.imageHelp}
              </span>
            </label>
            <input
              id={`${id}-images`}
              className="sr-only"
              type="file"
              multiple
              accept="image/jpeg,image/png,image/webp,image/gif"
              onChange={(event) => {
                if (event.target.files) appendFiles(event.target.files);
                event.target.value = "";
              }}
            />
            {values.images.length > 0 && (
              <div className="mt-2 grid gap-2" aria-label={dict.tasks.images}>
                {values.images.map((image, index) => (
                  <Attachment
                    key={image.id}
                    state={image.upload?.state ?? (image.key ? "done" : "idle")}
                    className="w-full flex-nowrap"
                  >
                    <AttachmentMedia variant="image">
                      {image.file ? (
                        <FileImagePreview
                          file={image.file}
                          name={image.file.name}
                        />
                      ) : image.url ? (
                        <img
                          src={image.url}
                          alt={`${dict.tasks.image} ${index + 1}`}
                          className="h-full w-full object-cover"
                        />
                      ) : null}
                    </AttachmentMedia>
                    <AttachmentContent>
                      <AttachmentTitle>
                        {image.file?.name ?? `${dict.tasks.image} ${index + 1}`}
                      </AttachmentTitle>
                      <AttachmentDescription
                        role="status"
                        className="whitespace-normal wrap-anywhere"
                      >
                        {image.upload?.state === "uploading"
                          ? `${dict.tasks.imageUploading}${image.upload.progress === undefined ? "" : ` · ${image.upload.progress}%`}`
                          : image.upload?.state === "processing"
                            ? dict.tasks.imageProcessing
                            : image.upload?.state === "error"
                              ? dict.tasks.imageUploadFailed
                              : image.key
                                ? dict.tasks.imageUploaded
                                : dict.tasks.imageReady}
                      </AttachmentDescription>
                      {image.upload?.state === "uploading" && (
                        <progress
                          className="mt-1 block h-1.5 w-full accent-primary"
                          max={100}
                          value={image.upload.progress}
                          aria-label={`${image.file?.name ?? dict.tasks.image}: ${dict.tasks.imageUploading}`}
                        />
                      )}
                    </AttachmentContent>
                    <AttachmentActions>
                      <AttachmentAction
                        type="button"
                        disabled={values.submitting}
                        aria-label={`${dict.tasks.removeImage} ${index + 1}`}
                        onClick={() =>
                          set.images(
                            values.images.filter(
                              (item) => item.id !== image.id,
                            ),
                          )
                        }
                      >
                        <X className="h-3.5 w-3.5" />
                      </AttachmentAction>
                    </AttachmentActions>
                  </Attachment>
                ))}
              </div>
            )}
            {imageError && (
              <p role="alert" className="mt-1 text-xs text-destructive">
                {imageError}
              </p>
            )}
          </div>
        </div>
      </details>
    </fieldset>
  );
}
