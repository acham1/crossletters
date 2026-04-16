const LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZ".split("");

export function BlankPicker({
  onConfirm,
  onCancel,
}: {
  onConfirm: (letter: string) => void;
  onCancel: () => void;
}) {
  return (
    <div className="modal" role="dialog" aria-label="Choose blank letter" onClick={onCancel}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h3>Choose a letter</h3>
        <div className="blank-grid">
          {LETTERS.map((L) => (
            <button key={L} onClick={() => onConfirm(L)}>
              {L}
            </button>
          ))}
        </div>
        <button className="btn-link" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}
