import Ajv, { type ErrorObject, type ValidateFunction } from "ajv";
import type { Diagnostic } from "@codemirror/lint";
import {
  isMap,
  isPair,
  isScalar,
  isSeq,
  parseDocument,
  type Pair,
  type ParsedNode,
} from "yaml";

import composeSchema from "$lib/schemas/compose-spec.json";

import { collectEnvDefinitions, isEnvFilePath } from "$lib/codemirror/env-lint";

const COMPOSE_SCHEMA_SOURCE_URL =
  "https://github.com/compose-spec/compose-spec/blob/main/schema/compose-spec.json";

const composeFilePattern = /^(docker-)?compose(?:\.[^.]+)?\.ya?ml$/i;
const interpolationPattern =
  /\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:-|-|:\?|\?|:\+|\+)([^}]*))?\}/g;

const validator = createComposeValidator();

type Range = {
  from: number;
  to: number;
};

export function isComposeFilePath(filePath: string): boolean {
  const name = filePath.split("/").pop() ?? filePath;
  return composeFilePattern.test(name);
}

export function composeSchemaSourceUrl(): string {
  return COMPOSE_SCHEMA_SOURCE_URL;
}

export function composeLinter(
  filePath: string,
  relatedFiles: Record<string, string>,
) {
  return (view: { state: { doc: { toString(): string } } }): Diagnostic[] => {
    const source = view.state.doc.toString();
    return validateComposeDocument(source, filePath, relatedFiles);
  };
}

function validateComposeDocument(
  source: string,
  filePath: string,
  relatedFiles: Record<string, string>,
): Diagnostic[] {
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
    return diagnostics;
  }

  const valid = validator(document.toJS());
  if (!valid && validator.errors) {
    for (const error of validator.errors) {
      diagnostics.push(
        diagnosticFromSchemaError(source, document.contents, error),
      );
    }
  }

  diagnostics.push(
    ...composeInterpolationDiagnostics(source, filePath, relatedFiles),
  );
  return diagnostics;
}

function createComposeValidator(): ValidateFunction {
  const ajv = new Ajv({
    allErrors: true,
    allowUnionTypes: true,
    strict: false,
    validateSchema: false,
  });

  return ajv.compile(composeSchema);
}

function composeInterpolationDiagnostics(
  source: string,
  filePath: string,
  relatedFiles: Record<string, string>,
): Diagnostic[] {
  const availableVariables = availableEnvVariables(filePath, relatedFiles);
  if (availableVariables.size === 0) {
    return [];
  }

  const diagnostics: Diagnostic[] = [];

  for (const match of source.matchAll(interpolationPattern)) {
    const variableName = match[1];
    const operator = match[2] ?? "";
    const index = match.index ?? 0;

    if (
      !requiresExistingVariable(operator) ||
      availableVariables.has(variableName)
    ) {
      continue;
    }

    diagnostics.push({
      from: index,
      to: index + match[0].length,
      severity: "warning",
      message: `Compose variable \`${variableName}\` is not defined in any open .env file from the same directory.`,
    });
  }

  return diagnostics;
}

function availableEnvVariables(
  filePath: string,
  relatedFiles: Record<string, string>,
): Set<string> {
  const composeDirectory = directoryName(filePath);
  const variables = new Set<string>();

  for (const [relatedPath, content] of Object.entries(relatedFiles)) {
    if (
      !isEnvFilePath(relatedPath) ||
      directoryName(relatedPath) !== composeDirectory
    ) {
      continue;
    }

    for (const definition of collectEnvDefinitions(content).definitions) {
      variables.add(definition.key);
    }
  }

  return variables;
}

function requiresExistingVariable(operator: string): boolean {
  return operator === "" || operator === "?" || operator === ":?";
}

function directoryName(filePath: string): string {
  const lastSlash = filePath.lastIndexOf("/");
  return lastSlash >= 0 ? filePath.slice(0, lastSlash) : "";
}

function diagnosticFromSchemaError(
  source: string,
  root: ParsedNode | null,
  error: ErrorObject,
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
  if (!pointer) {
    return [];
  }

  return pointer
    .split("/")
    .slice(1)
    .map((segment) => segment.replace(/~1/g, "/").replace(/~0/g, "~"));
}

function formatSchemaError(error: ErrorObject): string {
  if (
    error.keyword === "required" &&
    typeof error.params.missingProperty === "string"
  ) {
    return `Missing required property \`${error.params.missingProperty}\`.`;
  }

  if (
    error.keyword === "additionalProperties" &&
    typeof error.params.additionalProperty === "string"
  ) {
    return `Unknown property \`${error.params.additionalProperty}\` in Compose document.`;
  }

  if (error.instancePath) {
    return `${error.instancePath}: ${error.message ?? "Invalid Compose value."}`;
  }

  return error.message ?? "Invalid Compose document.";
}
