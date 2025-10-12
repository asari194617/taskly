type Role = "admin" | "manager" | "user";

export function Permission<T>({
  allow,
  role,
  children,
}: {
  allow: Role[];
  role: Role;
  children: T;
}) {
  return allow.includes(role) ? (children as T) : (undefined as unknown as T);
}
