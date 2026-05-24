import { dequal } from "dequal";
import * as v from "valibot";

const pageSchema = v.pipe(v.number(), v.integer(), v.minValue(1));
const textFilterSchema = v.pipe(v.string(), v.trim(), v.nonEmpty());

export type QueryFilterValues = Record<string, readonly string[]>;

export function parsePageParam(searchParams: URLSearchParams): number {
  const parsed = Number.parseInt(searchParams.get("page") ?? "1", 10);
  const result = v.safeParse(pageSchema, parsed);
  return result.success ? result.output : 1;
}

export function parseEnumFilterValues<
  const T extends readonly [string, ...string[]],
>(values: string[], allowedValues: T): T[number][] {
  const schema = v.picklist(allowedValues);
  return values.filter(
    (value): value is T[number] => v.safeParse(schema, value).success,
  );
}

export function parseTextFilterValues(values: string[]): string[] {
  return values.flatMap((value) => {
    const result = v.safeParse(textFilterSchema, value);
    return result.success ? [result.output] : [];
  });
}

export function buildFilterPageUrl(
  path: string,
  page: number,
  filters: QueryFilterValues,
): string {
  const params = new URLSearchParams();
  if (page > 1) {
    params.set("page", page.toString());
  }

  for (const [key, values] of Object.entries(filters)) {
    for (const value of values) {
      params.append(key, value);
    }
  }

  const query = params.toString();
  return query ? `${path}?${query}` : path;
}

export function filterValuesEqual(
  left: readonly unknown[],
  right: readonly unknown[],
): boolean {
  return dequal(left, right);
}
