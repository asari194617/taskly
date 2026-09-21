import { webFetch } from "@/lib/api";

const headers = {
  "Content-Type": "application/json",
  "X-Client-Platform": "web",
};

export async function saveNotificationSetting(
  apiBase: string,
  kind: "notification-methods" | "notification-types",
  value: Record<string, boolean>,
  request = webFetch,
) {
  const response = await request(`${apiBase}/settings/${kind}`, {
    method: "POST",
    headers,
    body: JSON.stringify(value),
  });
  if (!response.ok) throw new Error(`Failed to save ${kind}: ${response.status}`);
}

export async function applyNotificationSetting(
  previous: boolean,
  next: boolean,
  publish: (value: boolean) => void,
  persist: () => Promise<void>,
) {
  publish(next);
  try {
    await persist();
  } catch (error) {
    publish(previous);
    throw error;
  }
}
