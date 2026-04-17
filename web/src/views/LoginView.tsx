import { useEffect, useRef, useState } from "react";
import { api, type AuthConfig } from "../api/client";
import type { UserSummary } from "../api/types";

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string;
            callback: (response: { credential: string }) => void;
            auto_select?: boolean;
          }) => void;
          renderButton: (
            el: HTMLElement,
            options: { theme?: string; size?: string; width?: number },
          ) => void;
        };
      };
    };
  }
}

export function LoginView({ onLogin }: { onLogin: (u: UserSummary) => void }) {
  const [config, setConfig] = useState<AuthConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const googleBtnRef = useRef<HTMLDivElement>(null);

  // Dev login state
  const [userId, setUserId] = useState("");
  const [name, setName] = useState("");

  useEffect(() => {
    api.authConfig().then(setConfig).catch(() => {
      // If no config endpoint (old server), assume dev login only.
      setConfig({ devLogin: true });
    });
  }, []);

  // Load Google Identity Services script when we know the client ID.
  useEffect(() => {
    if (!config?.googleClientId) return;

    const handleCredential = async (response: { credential: string }) => {
      setBusy(true);
      setError(null);
      try {
        const u = await api.googleCallback(response.credential);
        onLogin(u);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setBusy(false);
      }
    };

    const initGoogle = () => {
      if (!window.google || !googleBtnRef.current) return;
      window.google.accounts.id.initialize({
        client_id: config.googleClientId!,
        callback: handleCredential,
        auto_select: true,
      });
      window.google.accounts.id.renderButton(googleBtnRef.current, {
        theme: "outline",
        size: "large",
        width: 300,
      });
    };

    // If the GIS script is already loaded, init now.
    if (window.google) {
      initGoogle();
      return;
    }

    const script = document.createElement("script");
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.onload = initGoogle;
    document.head.appendChild(script);
    return () => {
      document.head.removeChild(script);
    };
  }, [config?.googleClientId, onLogin]);

  const devSubmit = async (e: React.FormEvent) => {
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

  if (!config) {
    return <div className="center muted">Loading…</div>;
  }

  return (
    <div className="center">
      <div className="login-card">
        <h1>crossletters</h1>

        {config.googleClientId && (
          <>
            <div ref={googleBtnRef} style={{ minHeight: 44 }} />
            {busy && <p className="muted">Signing in…</p>}
          </>
        )}

        {config.devLogin && (
          <form onSubmit={devSubmit} style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
            {config.googleClientId && <hr style={{ border: "none", borderTop: "1px solid var(--panel-2)", margin: "0.5rem 0" }} />}
            <p className="muted" style={{ margin: 0 }}>
              {config.googleClientId ? "Or dev login:" : "Dev login"}
            </p>
            <input
              autoFocus={!config.googleClientId}
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              placeholder="User ID (e.g. alice)"
              required
            />
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Display name (optional)"
            />
            <button type="submit" disabled={busy || !userId.trim()}>
              {busy ? "…" : "Dev sign in"}
            </button>
          </form>
        )}

        {!config.devLogin && !config.googleClientId && (
          <p className="muted">No sign-in method configured.</p>
        )}

        {error && <div className="error">{error}</div>}
      </div>
    </div>
  );
}
