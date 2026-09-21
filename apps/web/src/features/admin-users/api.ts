import { gql } from "@/lib/graphql";

export type AdminUser = {
  id: number;
  email: string;
  display_name?: string | null;
  notification_lang?: string | null;
  email_login_enabled?: boolean | null;
  roles: string[];
  is_blocked: boolean;
};

export type Device = {
  id: number;
  user_id: number;
  platform: string;
  name?: string | null;
  payload?: string | null;
  is_enabled: boolean;
  created_at: string;
  updated_at: string;
};

export function loadAdminUsers(options: {
  query?: string;
  sortBy?: string;
  sortDir?: string;
  page: number;
  pageSize: number;
}) {
  return gql<{ adminUsersPaginated: { items: AdminUser[]; total: number } }>(
    `query($q:String,$sb:String,$sd:String,$p:Int,$ps:Int){ adminUsersPaginated(q:$q,sort_by:$sb,sort_dir:$sd,page:$p,page_size:$ps){ items{ id email display_name notification_lang email_login_enabled roles is_blocked } total } }`,
    { q: options.query, sb: options.sortBy, sd: options.sortDir, p: options.page, ps: options.pageSize },
  );
}

export function setBuiltInRole(userId: number, role: string, enabled: boolean) {
  const operation = enabled ? "adminUserAddRole" : "adminUserRemoveRole";
  return gql(`mutation($id:Int!,$r:String!){ ${operation}(user_id:$id, role:$r) }`, { id: userId, r: role });
}

export function blockUser(userId: number, reason?: string) {
  return gql(`mutation($id:Int!,$reason:String){ adminUserBlock(user_id:$id, reason:$reason) }`, { id: userId, reason });
}

export function unblockUser(userId: number) {
  return gql(`mutation($id:Int!){ adminUserUnblock(user_id:$id) }`, { id: userId });
}

export function updateUser(userId: number, input: { email?: string; displayName?: string; language?: string; emailLogin: boolean }) {
  return gql(`mutation($id:Int!,$email:String,$dn:String,$lang:String,$ema:Boolean){ adminUserUpdate(user_id:$id, email:$email, display_name:$dn, notification_lang:$lang, email_login_enabled:$ema) }`, {
    id: userId, email: input.email, dn: input.displayName, lang: input.language, ema: input.emailLogin,
  });
}

export function createUser(email: string, displayName?: string, language?: string) {
  return gql(`mutation($email:String!,$dn:String,$lang:String){ adminUserCreate(email:$email, display_name:$dn, notification_lang:$lang){ id } }`, { email, dn: displayName, lang: language });
}

export async function bulkCreateUsers(csv: string) {
  const result = await gql<{ adminUserBulkCreate: { created: number; errors: string[] } }>(
    `mutation($csv:String!){ adminUserBulkCreate(csv:$csv){ created errors } }`, { csv },
  );
  return result.adminUserBulkCreate;
}

export function setCustomRole(userId: number, role: string, enabled: boolean) {
  return gql(`mutation($id:Int!,$role:String!,$en:Boolean!){ adminUserSetCustomRole(user_id:$id, role:$role, enabled:$en) }`, { id: userId, role, en: enabled });
}

export function loadNotificationSettings(userId: number) {
  return gql<{
    adminUserNotificationSettings: { email_enabled: boolean; webpush_enabled: boolean };
    adminUserNotificationTypePrefs: { TASK: boolean; DAYSHIFT: boolean };
    adminUserDevices: Device[];
  }>(`query($id:Int!){
    adminUserNotificationSettings(user_id:$id){ email_enabled webpush_enabled }
    adminUserNotificationTypePrefs(user_id:$id){ TASK DAYSHIFT }
    adminUserDevices(user_id:$id){ id user_id platform name payload is_enabled created_at updated_at }
  }`, { id: userId });
}

export function saveNotificationChannels(userId: number, email: boolean, webpush: boolean) {
  return gql(`mutation($id:Int!,$em:Boolean!,$wp:Boolean!){ adminUserSetNotificationSettings(user_id:$id, email_enabled:$em, webpush_enabled:$wp) }`, { id: userId, em: email, wp: webpush });
}

export function saveNotificationType(userId: number, type: "TASK" | "DAYSHIFT", enabled: boolean) {
  return gql(`mutation($id:Int!,$tp:String!,$en:Boolean!){ adminUserSetNotificationTypePref(user_id:$id, notification_type:$tp, enabled:$en) }`, { id: userId, tp: type, en: enabled });
}

export async function loadDevices(userId: number) {
  const result = await gql<{ adminUserDevices: Device[] }>(
    `query($id:Int!){ adminUserDevices(user_id:$id){ id user_id platform name payload is_enabled created_at updated_at } }`, { id: userId },
  );
  return result.adminUserDevices ?? [];
}

export function updateDeviceEnabled(deviceId: number, enabled: boolean) {
  return gql(`mutation($id:Int!,$en:Boolean){ adminUserUpdateDevice(device_id:$id, is_enabled:$en) }`, { id: deviceId, en: enabled });
}

export function deleteDevice(deviceId: number) {
  return gql(`mutation($id:Int!){ adminUserDeleteDevice(device_id:$id) }`, { id: deviceId });
}

export function createDevice(userId: number, input: { platform: string; name?: string; payload?: string; enabled: boolean }) {
  return gql(`mutation($id:Int!,$pf:String!,$name:String,$payload:String,$en:Boolean){ adminUserCreateDevice(user_id:$id, platform:$pf, name:$name, payload:$payload, enabled:$en){ id } }`, {
    id: userId, pf: input.platform, name: input.name, payload: input.payload, en: input.enabled,
  });
}

export function deleteUser(userId: number, hard: boolean) {
  return gql(`mutation($id:Int!,$h:Boolean){ adminUserDelete(user_id:$id, hard:$h) }`, { id: userId, h: hard });
}
