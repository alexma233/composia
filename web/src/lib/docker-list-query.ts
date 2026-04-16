export const dockerListPageSize = 20;

export type DockerListSortDirection = "asc" | "desc";

export type DockerListPageQuery<SortField extends string> = {
  page: number;
  search: string;
  sortBy: SortField;
  sortDirection: DockerListSortDirection;
  sortDesc: boolean;
};

export function parseDockerListPageQuery<SortField extends string>(
  url: URL,
  allowedSortFields: readonly SortField[],
  defaultSortBy: SortField,
): DockerListPageQuery<SortField> {
  const page = parsePositivePage(url.searchParams.get("page"));
  const search = url.searchParams.get("search")?.trim() ?? "";
  const sortBy = parseSortField(
    url.searchParams.get("sortBy"),
    allowedSortFields,
    defaultSortBy,
  );
  const sortDirection = parseSortDirection(url.searchParams.get("sortDesc"));

  return {
    page,
    search,
    sortBy,
    sortDirection,
    sortDesc: sortDirection === "desc",
  };
}

export function buildDockerListPageUrl<SortField extends string>(
  path: string,
  query: Omit<DockerListPageQuery<SortField>, "sortDesc">,
  defaultSortBy: SortField,
): string {
  const params = new URLSearchParams();

  if (query.page > 1) {
    params.set("page", String(query.page));
  }
  if (query.search.trim()) {
    params.set("search", query.search.trim());
  }
  if (query.sortBy !== defaultSortBy) {
    params.set("sortBy", query.sortBy);
  }
  if (query.sortDirection === "desc") {
    params.set("sortDesc", "true");
  }

  const search = params.toString();
  return search ? `${path}?${search}` : path;
}

function parsePositivePage(value: string | null): number {
  const parsed = Number.parseInt(value ?? "", 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 1;
  }
  return parsed;
}

function parseSortField<SortField extends string>(
  value: string | null,
  allowedSortFields: readonly SortField[],
  defaultSortBy: SortField,
): SortField {
  if (!value) {
    return defaultSortBy;
  }
  return allowedSortFields.includes(value as SortField)
    ? (value as SortField)
    : defaultSortBy;
}

function parseSortDirection(value: string | null): DockerListSortDirection {
  return value === "true" ? "desc" : "asc";
}
