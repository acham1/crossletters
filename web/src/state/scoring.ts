// Client-side score preview. Mirrors internal/game/play.go closely enough to
// show a live score as the player drags tiles onto the board. The server is
// still authoritative — this only drives the UI badge.

import type { Board, PlacedTile } from "../api/types";
import type { PendingPlacement } from "../views/GameBoardView";
import { LETTER_VALUES, premiumAt } from "./premium";

export interface PreviewWord {
  word: string;
  score: number;
}

export interface ScorePreview {
  valid: boolean;
  reason?: string;
  words: PreviewWord[];
  total: number;
  bingo: boolean;
}

const RACK_SIZE = 7;
const BINGO_BONUS = 50;
const CENTER = 7;

type Cell = PlacedTile | null;

function tileValue(t: PlacedTile): number {
  if (t.blank) return 0;
  return LETTER_VALUES[t.letter.toUpperCase()] ?? 0;
}

function inBounds(r: number, c: number): boolean {
  return r >= 0 && r < 15 && c >= 0 && c < 15;
}

function boardIsEmpty(board: Board): boolean {
  for (let r = 0; r < 15; r++) {
    for (let c = 0; c < 15; c++) {
      if (board.squares[r][c]) return false;
    }
  }
  return true;
}

function makePreview(board: Board, pending: PendingPlacement[]): Cell[][] {
  const grid: Cell[][] = board.squares.map((row) => row.slice());
  for (const p of pending) {
    grid[p.row][p.col] = { letter: p.letter, blank: p.blank };
  }
  return grid;
}

function extractRun(
  grid: Cell[][],
  row: number,
  col: number,
  horiz: boolean,
): { word: string; cells: Array<[number, number]> } {
  const [dr, dc] = horiz ? [0, 1] : [1, 0];
  let r = row;
  let c = col;
  while (inBounds(r - dr, c - dc) && grid[r - dr][c - dc]) {
    r -= dr;
    c -= dc;
  }
  const cells: Array<[number, number]> = [];
  let word = "";
  while (inBounds(r, c) && grid[r][c]) {
    word += grid[r][c]!.letter;
    cells.push([r, c]);
    r += dr;
    c += dc;
  }
  return { word, cells };
}

export function previewScore(board: Board, pending: PendingPlacement[]): ScorePreview {
  if (pending.length === 0) {
    return { valid: false, words: [], total: 0, bingo: false };
  }
  if (pending.length > RACK_SIZE) {
    return { valid: false, reason: "too many tiles", words: [], total: 0, bingo: false };
  }

  // All placements in the same row or column.
  const row0 = pending[0].row;
  const col0 = pending[0].col;
  let horizOK = true;
  let vertOK = true;
  for (const p of pending.slice(1)) {
    if (p.row !== row0) horizOK = false;
    if (p.col !== col0) vertOK = false;
  }
  if (!horizOK && !vertOK) {
    return { valid: false, reason: "tiles not in line", words: [], total: 0, bingo: false };
  }

  const placedKey = (r: number, c: number) => `${r},${c}`;
  const placedSet = new Set(pending.map((p) => placedKey(p.row, p.col)));

  // First move must cover center; later moves must touch an existing tile.
  if (boardIsEmpty(board)) {
    if (!placedSet.has(placedKey(CENTER, CENTER))) {
      return { valid: false, reason: "first move must cover center", words: [], total: 0, bingo: false };
    }
  } else {
    let connected = false;
    for (const p of pending) {
      for (const [dr, dc] of [
        [-1, 0],
        [1, 0],
        [0, -1],
        [0, 1],
      ] as const) {
        const nr = p.row + dr;
        const nc = p.col + dc;
        if (inBounds(nr, nc) && board.squares[nr][nc]) {
          connected = true;
          break;
        }
      }
      if (connected) break;
    }
    if (!connected) {
      return { valid: false, reason: "play must connect to an existing tile", words: [], total: 0, bingo: false };
    }
  }

  const preview = makePreview(board, pending);

  // Main direction.
  let mainHoriz = horizOK;
  if (horizOK && vertOK) {
    // Single tile — pick whichever makes a >=2 run; horizontal wins ties.
    const p = pending[0];
    const hr = extractRun(preview, p.row, p.col, true);
    const vr = extractRun(preview, p.row, p.col, false);
    if (hr.cells.length >= 2) mainHoriz = true;
    else if (vr.cells.length >= 2) mainHoriz = false;
    else return { valid: false, reason: "no word of length ≥ 2", words: [], total: 0, bingo: false };
  }

  // Contiguity along main direction.
  if (pending.length >= 2) {
    let line: number;
    let lo: number;
    let hi: number;
    if (mainHoriz) {
      line = row0;
      lo = Math.min(...pending.map((p) => p.col));
      hi = Math.max(...pending.map((p) => p.col));
      for (let i = lo; i <= hi; i++) {
        if (!preview[line][i]) {
          return { valid: false, reason: "gap in word", words: [], total: 0, bingo: false };
        }
      }
    } else {
      line = col0;
      lo = Math.min(...pending.map((p) => p.row));
      hi = Math.max(...pending.map((p) => p.row));
      for (let i = lo; i <= hi; i++) {
        if (!preview[i][line]) {
          return { valid: false, reason: "gap in word", words: [], total: 0, bingo: false };
        }
      }
    }
  }

  // Collect main word + cross-words.
  type Run = { word: string; cells: Array<[number, number]>; horiz: boolean };
  const seen = new Set<string>();
  const runs: Run[] = [];
  const addRun = (r: Run) => {
    if (r.cells.length < 2) return;
    const key = `${r.horiz ? "h" : "v"}:${r.cells[0][0]}:${r.cells[0][1]}:${r.cells.length}`;
    if (seen.has(key)) return;
    seen.add(key);
    runs.push(r);
  };

  const main = extractRun(preview, pending[0].row, pending[0].col, mainHoriz);
  addRun({ ...main, horiz: mainHoriz });
  for (const p of pending) {
    const cross = extractRun(preview, p.row, p.col, !mainHoriz);
    addRun({ ...cross, horiz: !mainHoriz });
  }

  if (runs.length === 0) {
    return { valid: false, reason: "no word of length ≥ 2", words: [], total: 0, bingo: false };
  }

  // Score each run; premiums only apply on newly placed cells.
  const words: PreviewWord[] = [];
  let total = 0;
  for (const r of runs) {
    let sum = 0;
    let wordMult = 1;
    for (const [row, col] of r.cells) {
      const tile = preview[row][col]!;
      let letterVal = tileValue(tile);
      if (placedSet.has(placedKey(row, col))) {
        const prem = premiumAt(row, col);
        if (prem === "DL") letterVal *= 2;
        else if (prem === "TL") letterVal *= 3;
        else if (prem === "DW") wordMult *= 2;
        else if (prem === "TW") wordMult *= 3;
      }
      sum += letterVal;
    }
    const score = sum * wordMult;
    words.push({ word: r.word, score });
    total += score;
  }

  const bingo = pending.length === RACK_SIZE;
  if (bingo) total += BINGO_BONUS;

  return { valid: true, words, total, bingo };
}
