export type ConsistencyStatus =
  "unknown" | "consistent" | "drifted" | "error" | "not_applicable";

export type ConsistencyOutcome = {
  status: ConsistencyStatus;
  reasons: string[];
};

export type ServiceConsistency = {
  files: ConsistencyOutcome;
  compose: ConsistencyOutcome;
  checkedAt: string;
  repoRevision: string;
  taskId: string;
};

export type ServiceConsistencyResponse = {
  files?: { status?: string; reasons?: string[] };
  compose?: { status?: string; reasons?: string[] };
  checkedAt?: string;
  checked_at?: string;
  repoRevision?: string;
  repo_revision?: string;
  taskId?: string;
  task_id?: string;
};

export function latestServiceConsistency(
  current: ServiceConsistency | undefined,
  incoming: ServiceConsistency,
): ServiceConsistency {
  // Controller timestamps are UTC RFC3339Nano; dropping Z preserves fractional-second ordering.
  return current &&
    current.checkedAt.replace(/Z$/, "") > incoming.checkedAt.replace(/Z$/, "")
    ? current
    : incoming;
}

function outcome(
  value?: ServiceConsistencyResponse["files"],
): ConsistencyOutcome {
  const status = value?.status;
  return {
    status:
      status === "consistent" ||
      status === "drifted" ||
      status === "error" ||
      status === "not_applicable"
        ? status
        : "unknown",
    reasons: value?.reasons ?? [],
  };
}

export function serviceConsistency(
  value?: ServiceConsistencyResponse,
): ServiceConsistency {
  return {
    files: outcome(value?.files),
    compose: outcome(value?.compose),
    checkedAt: value?.checkedAt ?? value?.checked_at ?? "",
    repoRevision: value?.repoRevision ?? value?.repo_revision ?? "",
    taskId: value?.taskId ?? value?.task_id ?? "",
  };
}
