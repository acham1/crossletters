import { LETTER_VALUES } from "../state/premium";

// Tile is a visual-only rendering of a committed or pending tile.
export function Tile({
  letter,
  blank,
  pending,
}: {
  letter: string;
  blank: boolean;
  pending?: boolean;
}) {
  const value = blank ? 0 : (LETTER_VALUES[letter] ?? 0);
  return (
    <div className={`tile${pending ? " tile-pending" : ""}${blank ? " tile-blank" : ""}`}>
      <span className="tile-letter">{letter}</span>
      <span className="tile-value">{value}</span>
    </div>
  );
}
