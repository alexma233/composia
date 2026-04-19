import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

import { generateManifest, type Manifest } from "material-icon-theme";

type MaterialIconVariant = "default" | "light";

export type ResolvedMaterialIconNames = {
  iconName: string | null;
  lightIconName: string | null;
  expandedIconName: string | null;
  expandedLightIconName: string | null;
};

const require = createRequire(import.meta.url);
const materialIconThemeRoot = dirname(
  require.resolve("material-icon-theme/package.json"),
);
const materialIconDirectory = join(materialIconThemeRoot, "icons");
const svgCache = new Map<string, string>();

const defaultManifest = generateManifest();
const lightManifest = mergeManifest(defaultManifest, defaultManifest.light);
const knownIconNames = new Set<string>([
  ...Object.keys(defaultManifest.iconDefinitions ?? {}),
  ...Object.keys(lightManifest.iconDefinitions ?? {}),
]);

export function resolveMaterialIconNames(
  path: string,
  isDir: boolean,
): ResolvedMaterialIconNames {
  return {
    iconName: resolveIconName(path, isDir, "default", false),
    lightIconName: resolveIconName(path, isDir, "light", false),
    expandedIconName: isDir
      ? resolveIconName(path, true, "default", true)
      : null,
    expandedLightIconName: isDir
      ? resolveIconName(path, true, "light", true)
      : null,
  };
}

export async function loadMaterialIconSvg(iconName: string) {
  if (!knownIconNames.has(iconName)) {
    return null;
  }

  const cached = svgCache.get(iconName);
  if (cached) {
    return cached;
  }

  const svg = await readFile(join(materialIconDirectory, `${iconName}.svg`), {
    encoding: "utf8",
  });
  svgCache.set(iconName, svg);
  return svg;
}

function resolveIconName(
  path: string,
  isDir: boolean,
  variant: MaterialIconVariant,
  expanded: boolean,
) {
  const manifest = variant === "light" ? lightManifest : defaultManifest;
  return isDir
    ? resolveFolderIconName(path, manifest, expanded)
    : resolveFileIconName(path, manifest);
}

function resolveFileIconName(path: string, manifest: Manifest) {
  const normalizedPath = normalizePathKey(path);
  const fileName = lastPathSegment(normalizedPath);
  const fileNames = manifest.fileNames ?? {};

  for (const candidate of [normalizedPath, fileName]) {
    const iconName = fileNames[candidate];
    if (iconName) {
      return iconName;
    }
  }

  for (const extension of collectFileExtensions(fileName)) {
    const iconName = manifest.fileExtensions?.[extension];
    if (iconName) {
      return iconName;
    }
  }

  return manifest.file ?? null;
}

function resolveFolderIconName(
  path: string,
  manifest: Manifest,
  expanded: boolean,
) {
  const normalizedPath = normalizePathKey(path);
  const folderName = lastPathSegment(normalizedPath);
  const isRootFolder = !normalizedPath.includes("/");
  const exactMaps = isRootFolder
    ? [
        expanded ? manifest.rootFolderNamesExpanded : manifest.rootFolderNames,
        expanded ? manifest.folderNamesExpanded : manifest.folderNames,
      ]
    : [expanded ? manifest.folderNamesExpanded : manifest.folderNames];

  for (const map of exactMaps) {
    if (!map) {
      continue;
    }

    for (const candidate of [normalizedPath, folderName]) {
      const iconName = map[candidate];
      if (iconName) {
        return iconName;
      }
    }
  }

  if (isRootFolder) {
    return expanded
      ? (manifest.rootFolderExpanded ??
          manifest.folderExpanded ??
          manifest.rootFolder ??
          manifest.folder ??
          null)
      : (manifest.rootFolder ?? manifest.folder ?? null);
  }

  return expanded
    ? (manifest.folderExpanded ?? manifest.folder ?? null)
    : (manifest.folder ?? null);
}

function collectFileExtensions(fileName: string) {
  const parts = fileName.split(".");
  const extensions: string[] = [];

  for (let index = 1; index < parts.length; index += 1) {
    const extension = parts.slice(index).join(".");
    if (extension) {
      extensions.push(extension);
    }
  }

  return extensions;
}

function lastPathSegment(path: string) {
  const segments = path.split("/");
  return segments[segments.length - 1] ?? path;
}

function normalizePathKey(path: string) {
  return path.trim().replaceAll("\\", "/").toLowerCase();
}

function mergeManifest(base: Manifest, override?: Manifest): Manifest {
  return {
    ...base,
    ...override,
    fileNames: {
      ...(base.fileNames ?? {}),
      ...(override?.fileNames ?? {}),
    },
    fileExtensions: {
      ...(base.fileExtensions ?? {}),
      ...(override?.fileExtensions ?? {}),
    },
    folderNames: {
      ...(base.folderNames ?? {}),
      ...(override?.folderNames ?? {}),
    },
    folderNamesExpanded: {
      ...(base.folderNamesExpanded ?? {}),
      ...(override?.folderNamesExpanded ?? {}),
    },
    rootFolderNames: {
      ...(base.rootFolderNames ?? {}),
      ...(override?.rootFolderNames ?? {}),
    },
    rootFolderNamesExpanded: {
      ...(base.rootFolderNamesExpanded ?? {}),
      ...(override?.rootFolderNamesExpanded ?? {}),
    },
    iconDefinitions: {
      ...(base.iconDefinitions ?? {}),
      ...(override?.iconDefinitions ?? {}),
    },
  };
}
