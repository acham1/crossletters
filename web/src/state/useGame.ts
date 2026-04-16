import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import type { GameView } from "../api/types";

const POLL_MS = 8000;

// useGame polls the game state while the tab is visible. Returns the latest
// game, a manual refresh, and the most recent error.
export function useGame(gameId: string) {
  const [game, setGame] = useState<GameView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const pollRef = useRef<number | null>(null);

  const fetchOnce = useCallback(async () => {
    try {
      const g = await api.getGame(gameId);
      setGame(g);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [gameId]);

  useEffect(() => {
    fetchOnce();
    const schedule = () => {
      if (document.visibilityState !== "visible") return;
      pollRef.current = window.setTimeout(async () => {
        await fetchOnce();
        schedule();
      }, POLL_MS);
    };
    const stop = () => {
      if (pollRef.current) {
        window.clearTimeout(pollRef.current);
        pollRef.current = null;
      }
    };
    const onVis = () => {
      stop();
      if (document.visibilityState === "visible") {
        fetchOnce();
        schedule();
      }
    };
    document.addEventListener("visibilitychange", onVis);
    schedule();
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [fetchOnce]);

  return { game, error, loading, refresh: fetchOnce, setGame };
}
