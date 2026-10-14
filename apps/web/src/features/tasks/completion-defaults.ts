import { gql } from "@/lib/graphql";

export const taskTypeOptions = [
  { type: "ONLINE", label: "typeOnline" },
  { type: "FACE_TO_FACE", label: "typeFaceToFace" },
  { type: "QUIZ", label: "typeQuiz" },
  { type: "PRESENTATION", label: "typePresentation" },
  { type: "SELF_STUDY", label: "typeSelfStudy" },
  { type: "OTHER", label: "typeOther" },
] as const;

export type CompletionDefault = {
  type: string;
  is_completion_disabled: boolean;
};
export async function loadCompletionDefaults(): Promise<CompletionDefault[]> {
  const result = await gql<{ taskCompletionDefaults: CompletionDefault[] }>(
    `query { taskCompletionDefaults { type is_completion_disabled } }`,
  );
  return result.taskCompletionDefaults;
}

export function saveCompletionDefault(type: string, disabled: boolean) {
  return gql<{ adminSetTaskCompletionDefault: boolean }>(
    `mutation($type:String!,$disabled:Boolean!) { adminSetTaskCompletionDefault(type:$type,is_completion_disabled:$disabled) }`,
    { type, disabled },
  );
}
