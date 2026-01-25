import type { Message, WidgetState } from './types';
import { INITIAL_STATE } from './types';

/**
 * Simple state management for the widget
 * Uses a pub/sub pattern for reactivity
 */
export class StateManager {
  private state: WidgetState;
  private listeners: Set<(state: WidgetState) => void>;

  constructor() {
    this.state = { ...INITIAL_STATE };
    this.listeners = new Set();
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
    this.update({ sessionId });
  }

  setError(error: string | null): void {
    this.update({ error });
  }

  addMessage(message: Message): void {
    this.update({
      messages: [...this.state.messages, message],
    });
  }

  clearMessages(): void {
    this.update({
      messages: [],
      sessionId: null,
      error: null,
    });
  }

  reset(): void {
    this.state = { ...INITIAL_STATE };
    this.notify();
  }
}
