import { useDraggable, useDroppable } from "@dnd-kit/core";
import { Tile } from "./Tile";

export function RackStrip({
  rack,
  rackUsed,
  exchangeMode,
  exchangeSelection,
  onToggleExchange,
}: {
  rack: string[];
  rackUsed: Set<number>;
  exchangeMode: boolean;
  exchangeSelection: Set<number>;
  onToggleExchange: (idx: number) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: "rack-zone" });
  return (
    <div
      ref={setNodeRef}
      className={`rack${isOver ? " rack-over" : ""}${exchangeMode ? " rack-exchange" : ""}`}
      aria-label="Your tiles"
    >
      {rack.map((letter, idx) => {
        const used = rackUsed.has(idx);
        const selected = exchangeSelection.has(idx);
        if (used) {
          return <div key={idx} className="rack-slot empty" />;
        }
        if (exchangeMode) {
          return (
            <button
              key={idx}
              type="button"
              className={`rack-slot exchange${selected ? " selected" : ""}`}
              onClick={() => onToggleExchange(idx)}
            >
              <Tile letter={letter === "?" ? "?" : letter} blank={letter === "?"} />
            </button>
          );
        }
        return <DraggableRackTile key={idx} idx={idx} letter={letter} />;
      })}
    </div>
  );
}

function DraggableRackTile({ idx, letter }: { idx: number; letter: string }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: `rack-${idx}`,
  });
  const style = transform
    ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` }
    : undefined;
  const displayLetter = letter === "?" ? "?" : letter;
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className={`rack-slot tile-drag${isDragging ? " dragging" : ""}`}
    >
      <Tile letter={displayLetter} blank={letter === "?"} />
    </div>
  );
}
