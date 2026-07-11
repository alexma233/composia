import { describe, expect, test } from "bun:test";

import { formatBytes, formatDuration } from "./presenters.ts";

describe("presenters", () => {
  test("formats binary byte units", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1536)).toBe("1.5 KiB");
  });

  test("formats elapsed time", () => {
    const twoMinutesAgo = new Date(Date.now() - 120_000).toISOString();
    expect(formatDuration(twoMinutesAgo)).toBe("2 minutes");
  });
});
