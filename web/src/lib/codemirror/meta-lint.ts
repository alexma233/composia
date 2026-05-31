import Ajv, { type ErrorObject, type ValidateFunction } from "ajv";
import type { Diagnostic } from "@codemirror/lint";

import composiaMetaSchema from "$lib/schemas/composia-meta.json";
import { yamlSchemaDiagnostics } from "$lib/codemirror/yaml-schema-lint";

const metaFileName = "composia-meta.yaml";
const validator = createComposiaMetaValidator();

export function isComposiaMetaFilePath(filePath: string): boolean {
  return (filePath.split("/").pop() ?? filePath) === metaFileName;
}

export function composiaMetaLinter() {
  return (view: { state: { doc: { toString(): string } } }): Diagnostic[] => {
    return yamlSchemaDiagnostics(
      view.state.doc.toString(),
      validator,
      formatSchemaError,
    ).diagnostics;
  };
}

function createComposiaMetaValidator(): ValidateFunction {
  const ajv = new Ajv({
    allErrors: true,
    allowUnionTypes: true,
    strict: false,
    validateSchema: false,
  });

  return ajv.compile(composiaMetaSchema);
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
    return `Unknown property \`${error.params.additionalProperty}\` in composia-meta.yaml.`;
  }

  if (error.keyword === "enum" && Array.isArray(error.params.allowedValues)) {
    return `${schemaPath(error)} must be one of ${error.params.allowedValues
      .map((value) => `\`${String(value)}\``)
      .join(", ")}.`;
  }

  if (error.keyword === "minLength") {
    return `${schemaPath(error)} must not be empty.`;
  }

  if (error.keyword === "minItems") {
    return `${schemaPath(error)} must contain at least one item.`;
  }

  if (error.keyword === "uniqueItems") {
    return `${schemaPath(error)} must not contain duplicate items.`;
  }

  if (error.keyword === "oneOf") {
    return `${schemaPath(error)} must match exactly one allowed shape.`;
  }

  if (error.instancePath) {
    return `${error.instancePath}: ${error.message ?? "Invalid composia-meta value."}`;
  }

  return error.message ?? "Invalid composia-meta.yaml.";
}

function schemaPath(error: ErrorObject): string {
  return error.instancePath || "Document";
}
