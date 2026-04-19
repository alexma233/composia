import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

import {
  encryptedFileSourcePath,
  isEncryptedFilePath,
} from "$lib/service-workspace";
import {
  getIconForDirectoryPath,
  getIconForFilePath,
  isMaterialIconName,
  type MaterialIcon,
} from "vscode-material-icons";

export type ResolvedMaterialIconNames = {
  iconName: MaterialIcon;
  lightIconName: MaterialIcon | null;
  expandedIconName: MaterialIcon | null;
  expandedLightIconName: MaterialIcon | null;
};

const require = createRequire(import.meta.url);
const vscodeMaterialIconsRoot = dirname(
  dirname(require.resolve("vscode-material-icons")),
);
const materialIconDirectory = join(vscodeMaterialIconsRoot, "generated/icons");
const svgCache = new Map<string, string>();

export function resolveMaterialIconNames(
  path: string,
  isDir: boolean,
): ResolvedMaterialIconNames {
  const iconName = isDir
    ? getIconForDirectoryPath(path)
    : resolveFileIconName(path);
  const expandedIconName = isDir
    ? resolveVariantIconName(iconName, "-open")
    : null;

  return {
    iconName,
    lightIconName: resolveVariantIconName(iconName, "_light"),
    expandedIconName,
    expandedLightIconName: expandedIconName
      ? resolveVariantIconName(expandedIconName, "_light")
      : null,
  };
}

export async function loadVscodeMaterialIconSvg(iconName: string) {
  if (!isMaterialIconName(iconName)) {
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

function resolveFileIconName(path: string): MaterialIcon {
  const iconPath = isEncryptedFilePath(path)
    ? encryptedFileSourcePath(path)
    : path;
  const fileName = iconPath.split("/").pop() ?? iconPath;
  const extension = fileName.includes(".")
    ? fileName.slice(fileName.lastIndexOf(".") + 1).toLowerCase()
    : "";

  // vscode-material-icons currently misses the common yaml/yml extensions.
  if (extension === "yaml" || extension === "yml") {
    return "yaml";
  }

  return getIconForFilePath(iconPath);
}

function resolveVariantIconName(
  iconName: MaterialIcon,
  suffix: "-open" | "_light",
) {
  const candidate = `${iconName}${suffix}`;
  return isMaterialIconName(candidate) ? candidate : null;
}
