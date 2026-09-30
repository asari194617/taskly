import { gql } from "@/lib/graphql";

export type Cycle = {
  id: number;
  begin_date: string;
  finish_date: string;
  description: string;
};

export type Group = {
  id: number;
  cycle_id: number;
  position: number;
  user_ids: number[];
};

export type AdminUser = {
  id: number;
  email: string;
  display_name: string;
};

export async function loadCycles(page: number, pageSize: number) {
  return gql<{
    adminDayshiftCyclesPaginated: { items: Cycle[]; total: number };
    dayShiftCurrentCycle: number;
  }>(
    `query($page:Int,$size:Int){ adminDayshiftCyclesPaginated(page:$page,page_size:$size){ items{ id begin_date finish_date description } total } dayShiftCurrentCycle }`,
    { page, size: pageSize },
  );
}

export function loadGroups(cycleId: number, page: number, pageSize: number) {
  return gql<{
    adminDayshiftGroupsPaginated: { items: Group[]; total: number };
  }>(
    `query($cid:Int!,$page:Int,$size:Int){ adminDayshiftGroupsPaginated(cycle_id:$cid,page:$page,page_size:$size){ items{ id cycle_id position user_ids } total } }`,
    { cid: cycleId, page, size: pageSize },
  );
}

export async function loadUsersByIds(ids: number[]) {
  const result = await gql<{ adminUsersByIds: AdminUser[] }>(
    `query($ids:[Int!]!){ adminUsersByIds(ids:$ids){ id email display_name } }`,
    { ids },
  );
  return result.adminUsersByIds ?? [];
}

export async function loadDayShiftIndex() {
  const result = await gql<{ dayShiftIndex: number }>(`query{ dayShiftIndex }`);
  return result.dayShiftIndex ?? 0;
}

export async function loadCurrentCycleId() {
  try {
    const result = await gql<{ dayShiftCurrentCycle: number }>(
      `query{ dayShiftCurrentCycle }`,
    );
    return result.dayShiftCurrentCycle || 0;
  } catch {
    const legacy = await gql<{
      adminDayshiftCurrentCycle: { id: number } | null;
    }>(`query{ adminDayshiftCurrentCycle{ id } }`);
    return legacy.adminDayshiftCurrentCycle?.id || 0;
  }
}

export function createCycle(
  begin: string,
  finish: string,
  description: string,
) {
  return gql<{ adminDayshiftCreateCycle: { id: number } }>(
    `mutation($b:String!,$f:String!,$d:String!){ adminDayshiftCreateCycle(begin_date:$b, finish_date:$f, description:$d){ id } }`,
    { b: begin, f: finish, d: description },
  );
}

export function updateCycle(
  id: number,
  begin: string,
  finish: string,
  description: string,
) {
  return gql<{ adminDayshiftUpdateCycle: boolean }>(
    `mutation($id:Int!,$b:String!,$f:String!,$d:String!){ adminDayshiftUpdateCycle(id:$id, begin_date:$b, finish_date:$f, description:$d) }`,
    { id, b: begin, f: finish, d: description },
  );
}

export function deleteCycle(id: number) {
  return gql<{ adminDayshiftDeleteCycle: boolean }>(
    `mutation($id:Int!){ adminDayshiftDeleteCycle(id:$id) }`,
    { id },
  );
}

export function createGroup(cycleId: number, userIds: number[]) {
  return gql<{ adminDayshiftCreateGroup: { id: number } }>(
    `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftCreateGroup(cycle_id:$cid, user_ids:$ids){ id } }`,
    { cid: cycleId, ids: userIds },
  );
}

export function deleteGroup(cycleId: number, groupId: number) {
  return gql<{ adminDayshiftDeleteGroup: boolean }>(
    `mutation($cid:Int!,$gid:Int!){ adminDayshiftDeleteGroup(cycle_id:$cid, group_id:$gid) }`,
    { cid: cycleId, gid: groupId },
  );
}

export function reorderGroups(cycleId: number, groupIds: number[]) {
  return gql<{ adminDayshiftReorderGroups: boolean }>(
    `mutation($cid:Int!,$ids:[Int!]!){ adminDayshiftReorderGroups(cycle_id:$cid, group_ids:$ids) }`,
    { cid: cycleId, ids: groupIds },
  );
}

export async function loadGroupIds(cycleId: number) {
  const result = await gql<{ adminDayshiftGroupIds: number[] }>(
    `query($cid:Int!){ adminDayshiftGroupIds(cycle_id:$cid) }`,
    { cid: cycleId },
  );
  return result.adminDayshiftGroupIds ?? [];
}

export async function searchAdminUsers(query: string) {
  const result = await gql<{ adminUsers: AdminUser[] }>(
    `query($q:String){ adminUsers(q:$q){ id email display_name } }`,
    { q: query || undefined },
  );
  return result.adminUsers ?? [];
}

export function updateDayShiftIndex(index: number) {
  return gql<{ adminUpdateDayShiftIndex: boolean }>(
    `mutation($v:Int!){ adminUpdateDayShiftIndex(index:$v) }`,
    { v: index },
  );
}

export function setCurrentCycle(cycleId: number) {
  return gql<{ adminSetDayShiftCurrentCycle: boolean }>(
    `mutation($cid:Int!){ adminSetDayShiftCurrentCycle(cycle_id:$cid) }`,
    { cid: cycleId },
  );
}

export function updateGroupMembers(groupId: number, userIds: number[]) {
  return gql<{ adminDayshiftUpdateGroupMembers: boolean }>(
    `mutation($gid:Int!,$ids:[Int!]!){ adminDayshiftUpdateGroupMembers(group_id:$gid,user_ids:$ids) }`,
    { gid: groupId, ids: userIds },
  );
}
