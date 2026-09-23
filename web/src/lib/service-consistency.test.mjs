import { assertEquals } from "jsr:@std/assert@1.0.19/equals";
import {
  latestServiceConsistency,
  serviceConsistency,
} from "./service-consistency.ts";

Deno.test(
  "unchecked consistency has separate unknown stages and no provenance",
  () => {
    assertEquals(serviceConsistency(), {
      files: { status: "unknown", reasons: [] },
      compose: { status: "unknown", reasons: [] },
      checkedAt: "",
      repoRevision: "",
      taskId: "",
    });
  },
);

Deno.test(
  "consistency DTO preserves stage reasons and historical provenance in both RPC formats",
  () => {
    const outcomes = {
      files: { status: "consistent", reasons: [] },
      compose: { status: "drifted", reasons: ["Config hash differs"] },
    };
    const expected = {
      ...outcomes,
      checkedAt: "2026-09-22T12:00:00Z",
      repoRevision: "revision",
      taskId: "task-id",
    };
    assertEquals(serviceConsistency(expected), expected);
    assertEquals(
      serviceConsistency({
        ...outcomes,
        checked_at: expected.checkedAt,
        repo_revision: expected.repoRevision,
        task_id: expected.taskId,
      }),
      expected,
    );
  },
);

Deno.test(
  "late instance responses cannot replace a newer consistency snapshot",
  () => {
    const unchecked = serviceConsistency();
    const old = serviceConsistency({
      checkedAt: "2026-09-22T12:00:00Z",
      files: { status: "consistent" },
    });
    const recent = serviceConsistency({
      checkedAt: "2026-09-22T12:00:00.000000001Z",
      files: { status: "drifted", reasons: ["File changed"] },
    });
    const newest = serviceConsistency({
      checkedAt: "2026-09-22T12:00:01Z",
      compose: { status: "error" },
    });
    assertEquals(latestServiceConsistency(undefined, unchecked), unchecked);
    assertEquals(latestServiceConsistency(unchecked, old), old);
    assertEquals(latestServiceConsistency(recent, unchecked), recent);
    assertEquals(latestServiceConsistency(old, recent), recent);
    assertEquals(latestServiceConsistency(recent, old), recent);
    assertEquals(latestServiceConsistency(recent, newest), newest);
    assertEquals(latestServiceConsistency(newest, recent), newest);
  },
);

Deno.test("consistency statuses map independently to the UI labels", () => {
  for (const status of [
    "unknown",
    "consistent",
    "drifted",
    "error",
    "not_applicable",
  ]) {
    assertEquals(
      serviceConsistency({ files: { status } }).files.status,
      status,
    );
    assertEquals(
      serviceConsistency({ compose: { status } }).compose.status,
      status,
    );
  }
  assertEquals(
    serviceConsistency({ files: { status: "future-status" } }).files.status,
    "unknown",
  );
});
