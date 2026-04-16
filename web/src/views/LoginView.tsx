import { useState } from "react";
import { api } from "../api/client";
import type { UserSummary } from "../api/types";

// LoginView is the dev-mode login screen. In production this will be replaced
// by a Google Sign-In button that calls a different endpoint.
export function LoginView({ onLogin }: { onLogin: (u: UserSummary) => void }) {
  const [userId, setUserId] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const u = await api.devLogin(userId.trim(), name.trim() || userId.trim());
      onLogin(u);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="center">
      <form className="login-card" onSubmit={submit}>
        <h1>not-scrabble</h1>
        <p className="muted">
          Dev login. In production this will be Google Sign-In.
        </p>
        <label>
          User ID
          <input
            autoFocus
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            placeholder="e.g. alice"
            required
          />
        </label>
        <label>
          Display name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="optional"
          />
        </label>
        {error && <div className="error">{error}</div>}
        <button type="submit" disabled={busy || !userId.trim()}>
          {busy ? "…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
