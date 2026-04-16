import { useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import type { GameSummary } from "../api/types";

export function LobbyView({ onOpenGame }: { onOpenGame: (id: string) => void }) {
  const [games, setGames] = useState<GameSummary[] | null>(null);
  const [invite, setInvite] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = () => api.myGames().then(setGames).catch((e) => setError(String(e)));

  useEffect(() => {
    refresh();
  }, []);

  const create = async () => {
    setBusy(true);
    setError(null);
    try {
      const { gameId } = await api.createGame();
      onOpenGame(gameId);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const join = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const game = await api.joinGame({ inviteCode: invite.trim().toUpperCase() });
      onOpenGame(game.id);
    } catch (e) {
      if (e instanceof ApiError) setError(e.message);
      else setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="lobby">
      <section className="panel">
        <h2>New game</h2>
        <button onClick={create} disabled={busy}>
          Create game
        </button>
      </section>
      <section className="panel">
        <h2>Join by invite</h2>
        <form className="join-form" onSubmit={join}>
          <input
            value={invite}
            onChange={(e) => setInvite(e.target.value.toUpperCase())}
            placeholder="INVITE CODE"
            maxLength={8}
          />
          <button type="submit" disabled={busy || !invite.trim()}>
            Join
          </button>
        </form>
      </section>
      <section className="panel">
        <h2>Your games</h2>
        {error && <div className="error">{error}</div>}
        {games === null ? (
          <p className="muted">Loading…</p>
        ) : games.length === 0 ? (
          <p className="muted">No games yet. Create one above.</p>
        ) : (
          <ul className="game-list">
            {games.map((g) => (
              <li key={g.id}>
                <button className="game-link" onClick={() => onOpenGame(g.id)}>
                  <span className="game-players">{g.playerNames.join(", ")}</span>
                  <span className={`badge badge-${g.status}`}>{g.status}</span>
                  {g.yourTurn && <span className="badge badge-your-turn">Your turn</span>}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
