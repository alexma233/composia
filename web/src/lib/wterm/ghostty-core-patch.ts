import type { CellData, TerminalCore } from "@wterm/dom";

const DEFAULT_COLOR = 256;
const REVERSE_FLAG = 0x20;
const CELL_BYTES = 16;

const BLANK_CELL: CellData = {
  char: 32,
  fg: DEFAULT_COLOR,
  bg: DEFAULT_COLOR,
  flags: 0,
};

export interface TerminalThemeColors {
  background: number;
  foreground: number;
}

interface GhosttyWasmExports {
  memory: WebAssembly.Memory;
  get_scrollback_line(
    termPtr: number,
    offset: number,
    bufPtr: number,
    maxCols: number,
  ): number;
  alloc_buffer(length: number): number;
  free_buffer(ptr: number, length: number): void;
}

interface GhosttyCoreInternals {
  wasm?: { exports: GhosttyWasmExports };
  termPtr?: number;
}

interface WasmCellData {
  codepoint: number;
  fgR: number;
  fgG: number;
  fgB: number;
  bgR: number;
  bgG: number;
  bgB: number;
  flags: number;
  colorFlags: number;
}

function packRgb(r: number, g: number, b: number) {
  return (r << 16) | (g << 8) | b;
}

function parseCell(view: DataView, byteOffset: number): WasmCellData {
  return {
    codepoint: view.getUint32(byteOffset, true),
    fgR: view.getUint8(byteOffset + 4),
    fgG: view.getUint8(byteOffset + 5),
    fgB: view.getUint8(byteOffset + 6),
    bgR: view.getUint8(byteOffset + 7),
    bgG: view.getUint8(byteOffset + 8),
    bgB: view.getUint8(byteOffset + 9),
    flags: view.getUint8(byteOffset + 10),
    colorFlags: view.getUint8(byteOffset + 12),
  };
}

function readScrollbackCell(
  core: TerminalCore,
  offset: number,
  col: number,
): CellData | null {
  const internals = core as unknown as GhosttyCoreInternals;
  const wasm = internals.wasm;
  const termPtr = internals.termPtr;
  if (!wasm || !termPtr) {
    return null;
  }

  const maxCols = core.getCols();
  const lineSize = maxCols * CELL_BYTES;
  const bufPtr = wasm.exports.alloc_buffer(lineSize);
  if (bufPtr === 0) {
    return BLANK_CELL;
  }

  try {
    const length = wasm.exports.get_scrollback_line(
      termPtr,
      offset,
      bufPtr,
      maxCols,
    );
    if (length === 0 || col >= length) {
      return BLANK_CELL;
    }

    const view = new DataView(wasm.exports.memory.buffer, bufPtr, lineSize);
    const cell = parseCell(view, col * CELL_BYTES);
    if (cell.codepoint === 0 && cell.flags === 0 && cell.colorFlags === 0) {
      return BLANK_CELL;
    }

    const result: CellData = {
      char: cell.codepoint || 32,
      fg: DEFAULT_COLOR,
      bg: DEFAULT_COLOR,
      flags: cell.flags,
    };
    if (cell.colorFlags & 1) {
      result.fgRgb = packRgb(cell.fgR, cell.fgG, cell.fgB);
    }
    if (cell.colorFlags & 2) {
      result.bgRgb = packRgb(cell.bgR, cell.bgG, cell.bgB);
    }
    return result;
  } finally {
    wasm.exports.free_buffer(bufPtr, lineSize);
  }
}

function hasDefaultColors(cell: CellData) {
  return (
    cell.fg === DEFAULT_COLOR &&
    cell.bg === DEFAULT_COLOR &&
    cell.fgRgb === undefined &&
    cell.bgRgb === undefined
  );
}

function normalizeReverseDefaultCell(
  cell: CellData,
  colors: TerminalThemeColors,
): CellData {
  if (!(cell.flags & REVERSE_FLAG) || !hasDefaultColors(cell)) {
    return cell;
  }

  return {
    ...cell,
    flags: cell.flags & ~REVERSE_FLAG,
    fgRgb: colors.background,
    bgRgb: colors.foreground,
  };
}

export function patchGhosttyCore(
  core: TerminalCore,
  getThemeColors: () => TerminalThemeColors,
) {
  const originalGetCell = core.getCell.bind(core);
  const originalGetScrollbackCell = core.getScrollbackCell.bind(core);

  core.getCell = (row, col) =>
    normalizeReverseDefaultCell(originalGetCell(row, col), getThemeColors());
  core.getScrollbackCell = (offset, col) => {
    const cell =
      readScrollbackCell(core, offset, col) ??
      originalGetScrollbackCell(offset, col);
    return normalizeReverseDefaultCell(cell, getThemeColors());
  };
}
