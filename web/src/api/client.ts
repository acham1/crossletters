import type {
  CreateGameResponse,
  ErrorResponse,
  GameSummary,
  GameView,
  JoinRequest,
  PlayRequest,
  PlayResponse,
  UserSummary,
} from "./types";

export class ApiError extends Error {
  status: number;
  invalidWords?: string[];
  constructor(status: number, message: string, invalidWords?: string[]) {
    super(message);
    this.status = status;
    this.invalidWords = invalidWords;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    credentials: "include",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  };
  const resp = await fetch(path, init);
  const text = await resp.text();
  const data = text ? JSON.parse(text) : null;
  if (!resp.ok) {
    const err = data as ErrorResponse | null;
    throw new ApiError(resp.status, err?.error ?? resp.statusText, err?.invalidWords);
  }
  return data as T;
}

export const api = {
  devLogin: (userId: string, name: string) =>
    request<UserSummary>("POST", "/api/auth/dev/login", { userId, name }),
  devLogout: () => request<void>("POST", "/api/auth/dev/logout"),
  me: () => request<UserSummary>("GET", "/api/users/me"),
  myGames: () => request<GameSummary[]>("GET", "/api/users/me/games"),
  createGame: () => request<CreateGameResponse>("POST", "/api/games"),
  joinGame: (req: JoinRequest) => request<GameView>("POST", "/api/games/join", req),
  getGame: (id: string) => request<GameView>("GET", `/api/games/${id}`),
  startGame: (id: string) => request<GameView>("POST", `/api/games/${id}/start`),
  play: (id: string, req: PlayRequest) =>
    request<PlayResponse>("POST", `/api/games/${id}/plays`, req),
};
