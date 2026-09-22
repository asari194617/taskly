/** Local datetime values used by the token settings DateTimePicker. */
export class InvalidTokenExpiryError extends Error {}

const LOCAL_DATE_TIME =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/;
const pad = (value: number) => String(value).padStart(2, "0");

export function expiryToLocalInput(expiresAt?: string | null): string {
  if (!expiresAt) return "";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return "";
  const seconds = date.getSeconds();
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}${seconds ? `:${pad(seconds)}` : ""}`;
}

/**
 * Empty creation values omit the optional argument; empty updates send "" to
 * clear it. An untouched existing value keeps its exact instant, including
 * seconds, milliseconds, and the chosen side of a DST overlap.
 */
export function expiryFromLocalInput(
  input: string,
  options: { update?: boolean; original?: string | null } = {},
): string | undefined {
  if (!input.trim()) return options.update ? "" : undefined;
  if (options.original && input === expiryToLocalInput(options.original)) {
    return options.original;
  }
  const match = LOCAL_DATE_TIME.exec(input);
  if (!match) throw new InvalidTokenExpiryError();
  const [, year, month, day, hour, minute, second] = match;
  const parts = [year, month, day, hour, minute, second ?? "0"].map(Number);
  const [y, m, d, h, min, s] = parts;
  const date = new Date(0);
  date.setFullYear(y, m - 1, d);
  date.setHours(h, min, s, 0);
  if (
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== y ||
    date.getMonth() + 1 !== m ||
    date.getDate() !== d ||
    date.getHours() !== h ||
    date.getMinutes() !== min ||
    date.getSeconds() !== s
  ) {
    throw new InvalidTokenExpiryError();
  }
  return date.toISOString();
}
