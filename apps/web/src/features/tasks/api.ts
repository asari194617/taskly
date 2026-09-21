import { gql } from "@/lib/graphql";

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
  is_completed?: boolean;
  is_hidden?: boolean;
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
  deadline?: string;
  deadline_since?: string;
  deadline_until?: string;
  description?: string;
  type?: string;
};

const taskFields = `id title subject_id deadline deadline_since deadline_until description type is_require_hand_in is_self_check_required is_completed is_hidden`;
const taskMutationFields = `id title subject_id deadline deadline_since deadline_until description type is_require_hand_in is_self_check_required`;

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

export async function searchTasks(q: string, signal?: AbortSignal) {
  try {
    const result = await gql<{ tasks: { items: Task[] } }>(
      `query($q:String){ tasks(limit:200, q:$q){ items{ ${taskFields} } } }`,
      { q: q || undefined },
      { signal },
    );
    return result.tasks.items ?? [];
  } catch (error) {
    const message = error instanceof Error ? error.message : "";
    // Keep compatibility during rolling deployments with an older API schema.
    if (!/Cannot query field "is_completed"/i.test(message)) throw error;
    const result = await gql<{
      tasks: { items: Omit<Task, "is_completed">[] };
    }>(
      `query($q:String){ tasks(limit:200, q:$q){ items{ id title subject_id deadline deadline_since deadline_until description type is_require_hand_in is_self_check_required is_hidden } } }`,
      { q: q || undefined },
      { signal },
    );
    return (result.tasks.items ?? []).map((task) => ({
      ...task,
      is_completed: false,
    }));
  }
}

export async function createTask(input: TaskInput) {
  const result = await gql<{ createTask: Task }>(
    `mutation($title:String!,$subject_id:Int!,$is:Boolean,$scr:Boolean,$deadline:String!,$deadline_since:String,$deadline_until:String,$description:String,$type:String){
      createTask(title:$title, subject_id:$subject_id, is_require_hand_in:$is, is_self_check_required:$scr, deadline:$deadline, deadline_since:$deadline_since, deadline_until:$deadline_until, description:$description, type:$type){ ${taskMutationFields} }
    }`,
    input,
  );
  return result.createTask;
}

export function updateTask(id: number, input: Partial<TaskInput>) {
  return gql<{ updateTask: Pick<Task, "id"> }>(
    `mutation($id:Int!,$title:String,$subject_id:Int,$is:Boolean,$scr:Boolean,$deadline:String,$deadline_since:String,$deadline_until:String,$description:String,$type:String){
      updateTask(id:$id, title:$title, subject_id:$subject_id, is_require_hand_in:$is, is_self_check_required:$scr, deadline:$deadline, deadline_since:$deadline_since, deadline_until:$deadline_until, description:$description, type:$type){ id }
    }`,
    { id, ...input },
  );
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
