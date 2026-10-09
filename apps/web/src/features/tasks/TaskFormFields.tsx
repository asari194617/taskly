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
import type { Subject } from "./api";
import { useLang } from "@/react/LanguageProvider";

export type TaskFormValues = {
  title: string;
  subjectId: string;
  date: string;
  meta: DeadlineMeta | null;
  requireHandIn: boolean;
  selfCheck: boolean;
  type: string;
  description: string;
  imageFile?: File | null;
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
  imageFile?: (value: File | null) => void;
};

export function TaskFormFields({
  id,
  values,
  set,
  subjects,
  mode = "create",
}: {
  id: string;
  values: TaskFormValues;
  set: TaskFormSetters;
  subjects: Subject[];
  mode?: "create" | "edit";
}) {
  const { lang, dict } = useLang();
  const taskTypes = [
    dict.tasks.typeOnline,
    dict.tasks.typeFaceToFace,
    dict.tasks.typeQuiz,
    dict.tasks.typePresentation,
    dict.tasks.typeSelfStudy,
    dict.tasks.typeOther,
  ];
  return (
    <div className="grid grid-cols-2 gap-3">
      <div className="col-span-2">
        <Label htmlFor={`${id}-title`}>{dict.tasks.title}</Label>
        <Input
          id={`${id}-title`}
          value={values.title}
          onChange={(event) => set.title(event.target.value)}
          placeholder={dict.tasks.titlePlaceholder}
        />
      </div>
      <div>
        <Label htmlFor={`${id}-subject`}>{dict.tasks.subject}</Label>
        <SubjectSelect
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
          date={values.date}
          meta={values.meta}
          onChange={(date, meta) => {
            set.date(date);
            set.meta(meta);
          }}
        />
      </div>
      <div
        className={`col-span-2 flex items-center gap-2 ${mode === "edit" ? "order-last" : ""}`}
      >
        <Switch
          id={`${id}-req`}
          checked={values.requireHandIn}
          onCheckedChange={set.requireHandIn}
        />
        <Label htmlFor={`${id}-req`}>{dict.tasks.requireHandIn}</Label>
      </div>
      <div
        className={`col-span-2 flex items-center gap-2 -mt-2 ${mode === "edit" ? "order-last" : ""}`}
      >
        <Switch
          id={`${id}-scr`}
          checked={values.selfCheck}
          onCheckedChange={set.selfCheck}
        />
        <Label htmlFor={`${id}-scr`}>{dict.tasks.selfCheckRequired}</Label>
      </div>
      <div className={mode === "edit" ? "col-span-2" : undefined}>
        <Label htmlFor={`${id}-type`}>{dict.tasks.type}</Label>
        <Select value={values.type} onValueChange={set.type}>
          <SelectTrigger id={`${id}-type`}>
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
      <div className="col-span-2">
        <Label htmlFor={`${id}-desc`}>
          {dict.tasks.description} {dict.tasks.optional}
        </Label>
        <textarea
          id={`${id}-desc`}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm"
          rows={3}
          value={values.description}
          onChange={(event) => set.description(event.target.value)}
        />
      </div>
      {mode === "create" && (
        <div className="col-span-2">
          <Label htmlFor={`${id}-image`}>
            {dict.tasks.image} {dict.tasks.optional}
          </Label>
          <Input
            id={`${id}-image`}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            onChange={(event) => set.imageFile?.(event.target.files?.[0] ?? null)}
          />
          <p className="mt-1 text-xs text-muted-foreground">
            {dict.tasks.imageHelp}
          </p>
        </div>
      )}
    </div>
  );
}
