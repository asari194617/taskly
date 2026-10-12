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
} from "./api";
import { useLang } from "@/react/LanguageProvider";
import { ImagePlus, X } from "lucide-react";

export type TaskFormImage = {
  id: string;
  file?: File;
  key?: string;
  url?: string;
};

export type TaskFormValues = {
  title: string;
  subjectId: string;
  date: string;
  meta: DeadlineMeta | null;
  requireHandIn: boolean;
  selfCheck: boolean;
  type: string;
  description: string;
  images: TaskFormImage[];
};

export type TaskFormSetters = {
  title: (value: string) => void;
  subjectId: (value: string) => void;
  date: (value: string) => void;
  meta: (value: DeadlineMeta | null) => void;
  requireHandIn: (value: boolean) => void;
  selfCheck: (value: boolean) => void;
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
  const taskTypes = [
    dict.tasks.typeOnline,
    dict.tasks.typeFaceToFace,
    dict.tasks.typeQuiz,
    dict.tasks.typePresentation,
    dict.tasks.typeSelfStudy,
    dict.tasks.typeOther,
  ];
  const appendFiles = (files: FileList | File[]) => {
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
    <div
      className={`grid ${compact ? "grid-cols-1" : "grid-cols-2"} gap-4 [&>div]:min-w-0`}
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
                className={compact ? "h-11 w-full text-base" : undefined}
              >
                <SelectValue placeholder={dict.tasks.selectPlaceholder} />
              </SelectTrigger>
              <SelectContent>
                {taskTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
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
              className="mt-1 flex min-h-24 cursor-pointer flex-col items-center justify-center rounded-md border border-dashed bg-muted/30 px-4 py-3 text-center transition-colors hover:bg-muted/50"
              onDragOver={(event) => {
                event.preventDefault();
                event.dataTransfer.dropEffect = "copy";
              }}
              onDrop={(event) => {
                event.preventDefault();
                appendFiles(event.dataTransfer.files);
              }}
            >
              <ImagePlus className="mb-1 h-5 w-5 text-muted-foreground" />
              <span className="text-sm">{dict.tasks.imageDrop}</span>
              <span className="text-xs text-muted-foreground">
                {dict.tasks.imageHelp}
              </span>
            </label>
            <Input
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
              <div className="mt-2 grid grid-cols-3 gap-2 sm:grid-cols-5">
                {values.images.map((image, index) => (
                  <div
                    key={image.id}
                    className="group relative aspect-square overflow-hidden rounded-md border bg-muted"
                  >
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
                    <button
                      type="button"
                      className="absolute right-1 top-1 rounded-full bg-background/90 p-1 shadow"
                      aria-label={`${dict.tasks.removeImage} ${index + 1}`}
                      onClick={() =>
                        set.images(
                          values.images.filter((item) => item.id !== image.id),
                        )
                      }
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
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
    </div>
  );
}
