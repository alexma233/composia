import type { ErrorObject, ValidateFunction } from "ajv";
import type { Diagnostic } from "@codemirror/lint";
import { pointerSegments as jsonPointerSegments } from "@hyperjump/json-pointer";
import {
  isMap,
  isPair,
  isScalar,
  isSeq,
  parseDocument,
  type Pair,
  type ParsedNode,
} from "yaml";

export type Range = {
  from: number;
  to: number;
};

export type YamlSchemaDiagnosticsResult = {
  diagnostics: Diagnostic[];
  hasDocument: boolean;
  hasSyntaxErrors: boolean;
};

export function yamlSchemaDiagnostics(
  source: string,
  validator: ValidateFunction,
  formatSchemaError: (error: ErrorObject) => string,
): YamlSchemaDiagnosticsResult {
  const diagnostics: Diagnostic[] = [];
  const document = parseDocument(source, {
    prettyErrors: false,
    strict: false,
    uniqueKeys: false,
  });

  for (const error of document.errors) {
    diagnostics.push({
      ...rangeFromPositions(source, error.pos?.[0], error.pos?.[1]),
      severity: "error",
      message: error.message,
    });
  }

  if (document.errors.length > 0 || document.contents == null) {
    return {
      diagnostics,
      hasDocument: document.contents != null,
      hasSyntaxErrors: document.errors.length > 0,
    };
  }

  const valid = validator(document.toJS());
  if (!valid && validator.errors) {
    for (const error of validator.errors) {
      diagnostics.push(
        diagnosticFromSchemaError(
          source,
          document.contents,
          error,
          formatSchemaError,
        ),
      );
    }
  }

  return { diagnostics, hasDocument: true, hasSyntaxErrors: false };
}

function diagnosticFromSchemaError(
  source: string,
  root: ParsedNode | null,
  error: ErrorObject,
  formatSchemaError: (error: ErrorObject) => string,
): Diagnostic {
  const range = rangeForSchemaError(source, root, error);

  return {
    ...range,
    severity: "error",
    message: formatSchemaError(error),
  };
}

function rangeForSchemaError(
  source: string,
  root: ParsedNode | null,
  error: ErrorObject,
): Range {
  const pathSegments = pointerSegments(error.instancePath);

  if (error.keyword === "additionalProperties" && root) {
    const parentNode = nodeAtPath(root, pathSegments);
    const invalidKey = error.params.additionalProperty;

    if (typeof invalidKey === "string") {
      const pair = pairForKey(parentNode, invalidKey);
      const pairRange = rangeFromNode(source, pair?.key ?? pair);
      if (pairRange) {
        return pairRange;
      }
    }
  }

  const node = root ? nodeAtPath(root, pathSegments) : null;
  const nodeRange = rangeFromNode(source, node);
  if (nodeRange) {
    return nodeRange;
  }

  if (error.keyword === "required" && pathSegments.length > 0 && root) {
    const parentNode = nodeAtPath(root, pathSegments.slice(0, -1));
    const parentRange = rangeFromNode(source, parentNode);
    if (parentRange) {
      return parentRange;
    }
  }

  return { from: 0, to: Math.min(source.length, 1) };
}

function nodeAtPath(
  root: ParsedNode | null,
  pathSegments: string[],
): ParsedNode | Pair | null {
  let current: ParsedNode | Pair | null = root;

  for (const segment of pathSegments) {
    if (!current) {
      return null;
    }

    if (isPair(current)) {
      current = current.value as ParsedNode | null;
    }

    if (isMap(current)) {
      const pair = pairForKey(current, segment);
      current = (pair?.value ?? pair) as ParsedNode | Pair | null;
      continue;
    }

    if (isSeq(current)) {
      const index = Number(segment);
      if (!Number.isInteger(index)) {
        return null;
      }

      current = (current.items[index] ?? null) as ParsedNode | null;
      continue;
    }

    return null;
  }

  return current;
}

function pairForKey(node: unknown, key: string): Pair | null {
  if (!isMap(node)) {
    return null;
  }

  for (const item of node.items) {
    if (!isPair(item)) {
      continue;
    }

    if (scalarValue(item.key) === key) {
      return item;
    }
  }

  return null;
}

function scalarValue(node: unknown): string | null {
  if (isScalar(node)) {
    return node.value == null ? "" : String(node.value);
  }

  return null;
}

function rangeFromNode(source: string, node: unknown): Range | null {
  if (!node || typeof node !== "object" || !("range" in node)) {
    return null;
  }

  const range = (node as { range?: [number, number, number?] }).range;
  if (!range) {
    return null;
  }

  const [from, to] = range;
  if (typeof from !== "number" || typeof to !== "number") {
    return null;
  }

  return rangeFromPositions(source, from, to);
}

function rangeFromPositions(
  source: string,
  start?: number,
  end?: number,
): Range {
  const safeStart = typeof start === "number" ? Math.max(0, start) : 0;
  const safeEnd =
    typeof end === "number" ? Math.max(safeStart + 1, end) : safeStart + 1;
  const upperBound = source.length > 0 ? source.length : safeEnd;

  return {
    from: Math.min(safeStart, upperBound - (upperBound > 0 ? 1 : 0)),
    to: Math.min(Math.max(safeStart + 1, safeEnd), Math.max(upperBound, 1)),
  };
}

function pointerSegments(pointer: string): string[] {
  return [...jsonPointerSegments(pointer)];
}
