import type { Diagnostic } from "@codemirror/lint";
import { parse as parseEnvFile } from "envfile";

const envFilePattern = /^\.env(?:\.[^.]+)*$/i;

type Range = {
  from: number;
  to: number;
};

type EnvDefinition = {
  key: string;
  range: Range;
};

type EnvParseResult = {
  definitions: EnvDefinition[];
  diagnostics: Diagnostic[];
};

type EnvLine = {
  key: string;
  keyStart: number;
  value: string;
  valueStart: number;
};

export function isEnvFilePath(filePath: string): boolean {
  const name = filePath.split("/").pop() ?? filePath;
  return envFilePattern.test(name);
}

export function envLinter() {
  return (view: { state: { doc: { toString(): string } } }): Diagnostic[] => {
    return collectEnvDefinitions(view.state.doc.toString()).diagnostics;
  };
}

export function collectEnvDefinitions(source: string): EnvParseResult {
  const parsedEnv = parseEnvFile(source);
  const diagnostics: Diagnostic[] = [];
  const definitions: EnvDefinition[] = [];
  const seenKeys = new Map<string, Range>();

  let offset = 0;
  for (const line of source.split("\n")) {
    const lineRange = { from: offset, to: offset + line.length };
    const trimmed = line.trim();

    if (trimmed === "" || trimmed.startsWith("#")) {
      offset += line.length + 1;
      continue;
    }

    const parsedLine = parseEnvLine(line, offset);
    if (!parsedLine) {
      diagnostics.push({
        ...nonEmptyRange(lineRange),
        severity: "error",
        message:
          "Environment variables must use KEY=VALUE or KEY:VALUE syntax.",
      });
      offset += line.length + 1;
      continue;
    }

    const rawKey = parsedLine.key;
    const keyRange = {
      from: parsedLine.keyStart,
      to: parsedLine.keyStart + Math.max(rawKey.length, 1),
    };

    if (rawKey === "") {
      diagnostics.push({
        ...keyRange,
        severity: "error",
        message: "Environment variable name cannot be empty.",
      });
      offset += line.length + 1;
      continue;
    }

    if (seenKeys.has(rawKey)) {
      diagnostics.push({
        ...keyRange,
        severity: "warning",
        message: `Duplicate environment variable \`${rawKey}\`.`,
      });
    } else {
      seenKeys.set(rawKey, keyRange);
    }

    const quoteIssue = quoteDiagnostic(parsedLine.value, parsedLine.valueStart);
    if (quoteIssue) {
      diagnostics.push(quoteIssue);
    }

    const commentIssue = inlineCommentDiagnostic(
      parsedLine.value,
      parsedLine.valueStart,
    );
    if (commentIssue) {
      diagnostics.push(commentIssue);
    }

    if (Object.hasOwn(parsedEnv, rawKey)) {
      definitions.push({ key: rawKey, range: keyRange });
    }
    offset += line.length + 1;
  }

  return { definitions, diagnostics };
}

function parseEnvLine(line: string, offset: number): EnvLine | null {
  const match = line.match(/^([^=:#]+?)([=:])(.*)/);
  if (!match || match.index == null) {
    return null;
  }

  const rawKey = match[1] ?? "";
  const key = rawKey.trim();
  const keyPrefixOffset = rawKey.indexOf(key);
  const separatorOffset = rawKey.length;

  return {
    key,
    keyStart: offset + (keyPrefixOffset >= 0 ? keyPrefixOffset : 0),
    value: match[3] ?? "",
    valueStart: offset + separatorOffset + 1,
  };
}

function quoteDiagnostic(value: string, offset: number): Diagnostic | null {
  const trimmedStart = value.trimStart();
  const leadingWhitespace = value.length - trimmedStart.length;

  if (trimmedStart === "") {
    return null;
  }

  const quote = trimmedStart[0];
  if (quote !== '"' && quote !== "'") {
    return null;
  }

  for (let index = 1; index < trimmedStart.length; index += 1) {
    const character = trimmedStart[index];
    if (
      character === quote &&
      (quote !== '"' || trimmedStart[index - 1] !== "\\")
    ) {
      return null;
    }
  }

  return {
    from: offset + leadingWhitespace,
    to: offset + value.length,
    severity: "error",
    message: "Quoted environment value is missing a closing quote.",
  };
}

function inlineCommentDiagnostic(
  value: string,
  offset: number,
): Diagnostic | null {
  let quote: '"' | "'" | null = null;

  for (let index = 0; index < value.length; index += 1) {
    const character = value[index];

    if (quote) {
      if (character === quote && (quote !== '"' || value[index - 1] !== "\\")) {
        quote = null;
      }
      continue;
    }

    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }

    if (character === "#" && index > 0 && value[index - 1] !== " ") {
      return {
        from: offset + index,
        to: offset + index + 1,
        severity: "warning",
        message:
          "Unquoted values that contain # should add a space before the comment or wrap the value in quotes.",
      };
    }
  }

  return null;
}

function nonEmptyRange(range: Range): Range {
  if (range.from === range.to) {
    return { from: range.from, to: range.to + 1 };
  }

  return range;
}
