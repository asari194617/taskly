import { afterEach, describe, expect, it } from "vitest";
import {
  expiryFromLocalInput,
  expiryToLocalInput,
  InvalidTokenExpiryError,
} from "./token-expiry";

const initialTimeZone = process.env.TZ;
afterEach(() => {
  process.env.TZ = initialTimeZone;
});

describe("token expiration datetime conversion", () => {
  it("distinguishes no expiration on create from clearing it on update", () => {
    expect(expiryFromLocalInput("")).toBeUndefined();
    expect(expiryFromLocalInput("  ", { update: true })).toBe("");
    expect(expiryToLocalInput(null)).toBe("");
  });

  it("converts local dates across time zones and date boundaries", () => {
    process.env.TZ = "Asia/Tokyo";
    expect(expiryFromLocalInput("2026-01-01T00:30")).toBe("2025-12-31T15:30:00.000Z");
    expect(expiryToLocalInput("2025-12-31T15:30:00Z")).toBe("2026-01-01T00:30");
    process.env.TZ = "America/New_York";
    expect(expiryFromLocalInput("2026-12-31T23:45:30")).toBe("2027-01-01T04:45:30.000Z");
    expect(expiryToLocalInput("2027-01-01T04:45:30Z")).toBe("2026-12-31T23:45:30");
  });

  it.each(["2026-02-30T12:00", "2026-01-01T24:00", "2026-01-01T12:60", "2026-01-01", "tomorrow"])(
    "rejects invalid local datetime %s",
    (input) => expect(() => expiryFromLocalInput(input)).toThrow(InvalidTokenExpiryError),
  );

  it("rejects a nonexistent DST time", () => {
    process.env.TZ = "America/New_York";
    expect(() => expiryFromLocalInput("2026-03-08T02:30")).toThrow(InvalidTokenExpiryError);
  });

  it("preserves the exact instant when an existing expiration is reopened and saved", () => {
    process.env.TZ = "America/New_York";
    const original = "2026-11-01T06:30:15.123Z";
    const local = expiryToLocalInput(original);
    expect(local).toBe("2026-11-01T01:30:15");
    expect(expiryFromLocalInput(local, { update: true, original })).toBe(original);
    expect(expiryFromLocalInput("2026-11-01T01:45:15", { update: true, original })).toBe("2026-11-01T05:45:15.000Z");
  });
});
