import Ajv, { type ErrorObject, type ValidateFunction } from "ajv";
import type { Diagnostic } from "@codemirror/lint";

import composeSchema from "$lib/schemas/compose-spec.json";

import { collectEnvDefinitions, isEnvFilePath } from "$lib/codemirror/env-lint";
import { yamlSchemaDiagnostics } from "$lib/codemirror/yaml-schema-lint";

const COMPOSE_SCHEMA_SOURCE_URL =
  "https://github.com/compose-spec/compose-spec/blob/main/schema/compose-spec.json";

const composeFilePattern = /^(docker-)?compose(?:\.[^.]+)?\.ya?ml$/i;
const interpolationPattern =
  /\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:-|-|:\?|\?|:\+|\+)([^}]*))?\}/g;

const validator = createComposeValidator();

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
  const schemaResult = yamlSchemaDiagnostics(
    source,
    validator,
    formatSchemaError,
  );
  const diagnostics = [...schemaResult.diagnostics];

  if (schemaResult.hasSyntaxErrors || !schemaResult.hasDocument) {
    return diagnostics;
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
