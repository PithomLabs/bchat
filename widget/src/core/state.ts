import type { Message, WidgetState, BridgeState } from './types';
import { INITIAL_STATE } from './types';

/**
 * Simple state management for the widget
 * Uses a pub/sub pattern for reactivity
 */
export class StateManager {
  private state: WidgetState;
  private listeners: Set<(state: WidgetState) => void>;
  private tenantSlug: string;

  constructor(tenantSlug: string) {
    this.state = { ...INITIAL_STATE };
    this.listeners = new Set();
    this.tenantSlug = tenantSlug;
  }

  /**
   * Get current state
   */
  getState(): Readonly<WidgetState> {
    return this.state;
  }

  /**
   * Subscribe to state changes
   */
  subscribe(listener: (state: WidgetState) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  /**
   * Notify all listeners of state change
   */
  private notify(): void {
    const currentState = this.state;
    this.listeners.forEach((listener) => listener(currentState));
  }

  /**
   * Update state and notify listeners
   */
  private update(partial: Partial<WidgetState>): void {
    this.state = { ...this.state, ...partial };
    this.notify();
  }

  // State actions

  toggleOpen(): void {
    this.update({ isOpen: !this.state.isOpen });
  }

  setOpen(isOpen: boolean): void {
    this.update({ isOpen });
  }

  toggleMinimized(): void {
    this.update({ isMinimized: !this.state.isMinimized });
  }

  setMinimized(isMinimized: boolean): void {
    this.update({ isMinimized });
  }

  setLoading(isLoading: boolean): void {
    this.update({ isLoading });
  }

  setSessionId(sessionId: string | null): void {
    const key = `bchat_session_id:${this.tenantSlug}`;
    if (sessionId) {
      localStorage.setItem(key, sessionId);
    } else {
      localStorage.removeItem(key);
    }
    this.update({ sessionId });
  }

  private generateUUID(): string {
    try {
      if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
        return crypto.randomUUID();
      }
    } catch { /* fall through */ }
    const rnds = new Uint8Array(16);
    if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
      crypto.getRandomValues(rnds);
    } else {
      for (let i = 0; i < 16; i++) rnds[i] = Math.floor(Math.random() * 256);
    }
    rnds[6] = (rnds[6] & 0x0f) | 0x40;
    rnds[8] = (rnds[8] & 0x3f) | 0x80;
    const hex = Array.from(rnds).map(b => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-4${hex.slice(13, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }

  getOrCreatePendingMessageID(message: string): string {
    const key = `bchat_pending:${this.tenantSlug}`;
    let map: Record<string, string> = {};
    try {
      const stored = localStorage.getItem(key);
      if (stored) {
        const parsed = JSON.parse(stored);
        if (parsed && typeof parsed === "object") map = parsed;
      }
    } catch {
      localStorage.removeItem(key);
    }

    if (map[message]) return map[message];
    const id = this.generateUUID();
    map[message] = id;
    try {
      localStorage.setItem(key, JSON.stringify(map));
    } catch { /* quota exceeded; proceed without persistence */ }
    return id;
  }

  acknowledgePendingMessage(message: string, clientMessageId: string): void {
    const key = `bchat_pending:${this.tenantSlug}`;
    let map: Record<string, string> = {};
    try {
      const stored = localStorage.getItem(key);
      if (stored) {
        const parsed = JSON.parse(stored);
        if (parsed && typeof parsed === "object") map = parsed;
      }
    } catch {
      localStorage.removeItem(key);
      return;
    }
    if (map[message] === clientMessageId) {
      delete map[message];
      try {
        if (Object.keys(map).length === 0) localStorage.removeItem(key);
        else localStorage.setItem(key, JSON.stringify(map));
      } catch { /* ignore */ }
    }
  }

  setError(error: string | null): void {
    this.update({ error });
  }

  setBridge(bridge: BridgeState | null): void {
    this.update({ bridge });
  }

  setMessages(messages: Message[]): void {
    this.update({ messages });
  }

  addMessage(message: Message): void {
    this.update({
      messages: [...this.state.messages, message],
    });
  }

  clearMessages(): void {
    const key = `bchat_session_id:${this.tenantSlug}`;
    localStorage.removeItem(key);
    localStorage.removeItem(`bchat_pending:${this.tenantSlug}`);
    this.update({
      messages: [],
      sessionId: null,
      error: null,
      bridge: null,
    });
  }

  reset(): void {
    this.state = { ...INITIAL_STATE };
    this.notify();
  }
}
