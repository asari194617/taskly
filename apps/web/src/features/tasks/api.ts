import { gql } from "@/lib/graphql";
import { getApiBase, webFetch } from "@/lib/api";

export type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline?: string | null;
  deadline_since?: string | null;
  deadline_until?: string | null;
  description?: string | null;
  type?: string | null;
  is_require_hand_in?: boolean;
  is_self_check_required?: boolean;
  is_completion_disabled?: boolean;
  is_completed?: boolean;
  is_hidden?: boolean;
  image_url?: string | null;
  image_keys?: string[];
  image_urls?: string[];
};

export type Subject = {
  id: number;
  name_ja: string;
  name_en?: string | null;
};

export type TaskInput = {
  title: string;
  subject_id: number;
  is?: boolean;
  scr?: boolean;
  completionDisabled?: boolean;
  deadline?: string;
  deadline_since?: string;
  deadline_until?: string;
  description?: string;
  type?: string;
  image_key?: string;
  image_keys?: string[];
};

const taskFields = `id title subject_id deadline deadline_since deadline_until description image_url image_keys image_urls type is_require_hand_in is_self_check_required is_completion_disabled is_completed is_hidden`;
const taskMutationFields = `id title subject_id deadline deadline_since deadline_until description image_url image_keys image_urls type is_require_hand_in is_self_check_required is_completion_disabled`;

export const TASK_IMAGE_MAX_BYTES = 10 * 1024 * 1024;
export const TASK_IMAGE_MAX_COUNT = 5;
export const TASK_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp", "image/gif"];

export async function uploadTaskImage(file: File): Promise<string> {
  if (!TASK_IMAGE_TYPES.includes(file.type)) {
    throw new Error("unsupported_image_type");
  }
  if (file.size <= 0 || file.size > TASK_IMAGE_MAX_BYTES) {
    throw new Error("image_too_large");
  }
  const presignResponse = await webFetch(`${getApiBase()}/task-images/presign`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content_type: file.type, size: file.size }),
  });
  const presign = await presignResponse.json();
  if (!presignResponse.ok) {
    throw new Error(presign?.error || "image_presign_failed");
  }
  const uploadResponse = await fetch(presign.upload_url, {
    method: "PUT",
    headers: presign.headers,
    body: file,
  });
  if (!uploadResponse.ok) {
    throw new Error("image_upload_failed");
  }
  return presign.key;
}

export async function loadSubjects() {
  const result = await gql<{ subjects: Subject[] }>(
    `query{ subjects{ id name_ja name_en } }`,
  );
  return result.subjects ?? [];
}

export async function loadViewerRole() {
  const result = await gql<{
    me: { is_admin: boolean; is_manager: boolean };
  }>(`query{ me{ is_admin is_manager } }`);
  return {
    isAdmin: !!result.me?.is_admin,
    isManager: !!result.me?.is_manager,
  };
}

export type TaskCursor = { cursor_deadline: string; cursor_id: string };
export type TaskPage = { items: Task[]; nextCursor: TaskCursor | null };

export async function searchTasks(q: string, signal?: AbortSignal, cursor?: TaskCursor): Promise<TaskPage> {
  const variables = { q: q || undefined, ...cursor };
  try {
    const result = await gql<{ tasks: { items: Task[]; next_cursor: TaskCursor | null } }>(
      `query($q:String,$cursor_deadline:String,$cursor_id:String){ tasks(limit:15, q:$q, cursor_deadline:$cursor_deadline, cursor_id:$cursor_id){ items{ ${taskFields} } next_cursor{ cursor_deadline cursor_id } } }`,
      variables,
      { signal },
    );
    return { items: result.tasks.items ?? [], nextCursor: result.tasks.next_cursor ?? null };
  } catch (error) {
    const message = error instanceof Error ? error.message : "";
    // Keep compatibility during rolling deployments with an older API schema.
    if (!/Cannot query field "(is_completion_disabled|is_completed|image_url|image_keys|image_urls)"/i.test(message)) throw error;
    const result = await gql<{
      tasks: { items: Omit<Task, "is_completed">[]; next_cursor: TaskCursor | null };
    }>(
      `query($q:String,$cursor_deadline:String,$cursor_id:String){ tasks(limit:15, q:$q, cursor_deadline:$cursor_deadline, cursor_id:$cursor_id){ items{ id title subject_id deadline deadline_since deadline_until description type is_require_hand_in is_self_check_required is_hidden } next_cursor{ cursor_deadline cursor_id } } }`,
      variables,
      { signal },
    );
    return {
      items: (result.tasks.items ?? []).map((task) => ({ ...task, is_completed: false })),
      nextCursor: result.tasks.next_cursor ?? null,
    };
  }
}

export async function createTask(input: TaskInput) {
  const result = await gql<{ createTask: Task }>(
    `mutation($title:String!,$subject_id:Int!,$is:Boolean,$scr:Boolean,$completionDisabled:Boolean,$deadline:String!,$deadline_since:String,$deadline_until:String,$description:String,$type:String,$image_keys:[String!]){
      createTask(title:$title, subject_id:$subject_id, is_require_hand_in:$is, is_self_check_required:$scr, is_completion_disabled:$completionDisabled, deadline:$deadline, deadline_since:$deadline_since, deadline_until:$deadline_until, description:$description, type:$type, image_keys:$image_keys){ ${taskMutationFields} }
    }`,
    input,
  );
  return result.createTask;
}

export async function updateTask(id: number, input: Partial<TaskInput>) {
  const result = await gql<{ updateTask: Task }>(
    `mutation($id:Int!,$title:String,$subject_id:Int,$is:Boolean,$scr:Boolean,$completionDisabled:Boolean,$deadline:String,$deadline_since:String,$deadline_until:String,$description:String,$type:String,$image_keys:[String!]){
      updateTask(id:$id, title:$title, subject_id:$subject_id, is_require_hand_in:$is, is_self_check_required:$scr, is_completion_disabled:$completionDisabled, deadline:$deadline, deadline_since:$deadline_since, deadline_until:$deadline_until, description:$description, type:$type, image_keys:$image_keys){ ${taskMutationFields} }
    }`,
    { id, ...input },
  );
  return result.updateTask;
}

export function deleteTask(id: number) {
  return gql<{ deleteTask: boolean }>(
    `mutation($id:Int!){ deleteTask(id:$id) }`,
    { id },
  );
}

export function setTaskCompleted(id: number, completed: boolean) {
  return completed
    ? gql<{ completeTask: boolean }>(
        `mutation($id:Int!){ completeTask(id:$id) }`,
        { id },
      )
    : gql<{ uncompleteTask: boolean }>(
        `mutation($id:Int!){ uncompleteTask(id:$id) }`,
        { id },
      );
}

export function reportTask(taskId: number, reason: string, comment?: string) {
  return gql<{ createUserReport: { id: number } }>(
    `mutation($task_id:Int!,$reason:String!,$comment:String){ createUserReport(task_id:$task_id, reason:$reason, comment:$comment){ id } }`,
    { task_id: taskId, reason, comment },
  );
}
