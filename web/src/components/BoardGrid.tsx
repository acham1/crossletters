import { useDraggable, useDroppable } from "@dnd-kit/core";
import type { Board } from "../api/types";
import { premiumAt, type Premium } from "../state/premium";
import { Tile } from "./Tile";
import type { PendingPlacement } from "../views/GameBoardView";

export function BoardGrid({
  board,
  pending,
  onCellTap,
  selectedRackIdx,
}: {
  board: Board;
  pending: PendingPlacement[];
  onCellTap?: (row: number, col: number) => void;
  selectedRackIdx?: number | null;
}) {
  const pendingMap = new Map<string, PendingPlacement>();
  for (const p of pending) pendingMap.set(`${p.row},${p.col}`, p);

  return (
    <div className="board" role="grid" aria-label="Scrabble board">
      {Array.from({ length: 15 }).map((_, row) => (
        <div key={row} className="board-row" role="row">
          {Array.from({ length: 15 }).map((_, col) => {
            const committed = board.squares[row]?.[col] ?? null;
            const pendingTile = pendingMap.get(`${row},${col}`);
            return (
              <BoardCell
                key={col}
                row={row}
                col={col}
                committed={committed}
                pending={pendingTile}
                premium={premiumAt(row, col)}
                tappable={selectedRackIdx != null && !committed && !pendingTile}
                onTap={onCellTap}
              />
            );
          })}
        </div>
      ))}
    </div>
  );
}

function BoardCell({
  row,
  col,
  committed,
  pending,
  premium,
  tappable,
  onTap,
}: {
  row: number;
  col: number;
  committed: { letter: string; blank: boolean } | null;
  pending: PendingPlacement | undefined;
  premium: Premium;
  tappable?: boolean;
  onTap?: (row: number, col: number) => void;
}) {
  const { setNodeRef: dropRef, isOver } = useDroppable({ id: `cell-${row}-${col}` });
  const hasTile = committed || pending;

  return (
    <div
      ref={dropRef}
      role="gridcell"
      onClick={tappable && onTap ? () => onTap(row, col) : undefined}
      className={[
        "cell",
        `premium-${premium}`,
        hasTile ? "has-tile" : "",
        isOver ? "cell-over" : "",
        tappable ? "cell-tappable" : "",
        row === 7 && col === 7 ? "center-star" : "",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {committed && <Tile letter={committed.letter} blank={committed.blank} />}
      {pending && <DraggablePending placement={pending} />}
      {!hasTile && <PremiumBadge premium={premium} isCenter={row === 7 && col === 7} />}
    </div>
  );
}

function DraggablePending({ placement }: { placement: PendingPlacement }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: `pending-${placement.row}-${placement.col}`,
  });
  const style = transform
    ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` }
    : undefined;
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className={`tile-drag${isDragging ? " dragging" : ""}`}
    >
      <Tile letter={placement.letter} blank={placement.blank} pending />
    </div>
  );
}

function PremiumBadge({ premium, isCenter }: { premium: Premium; isCenter: boolean }) {
  if (isCenter) return <span className="premium-label">★</span>;
  if (premium === "none") return null;
  const label =
    premium === "DL" ? "DL" : premium === "TL" ? "TL" : premium === "DW" ? "DW" : "TW";
  return <span className="premium-label">{label}</span>;
}
