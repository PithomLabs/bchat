import type { WidgetConfig, WidgetState } from '../core/types';
import { sendMessage, RateLimitError } from '../core/api';
import { StateManager } from '../core/state';
import { getStyles } from '../styles/styles';
import { createButton, updateButtonVisibility } from './Button';
import { createPanel, updatePanelOpen, updatePanelMinimized } from './Panel';
import { createMessagesContainer, updateMessages } from './Messages';
import { createInputArea, updateInputDisabled, focusInput } from './Input';

/**
 * Main Widget class that orchestrates all components
 */
export class Widget {
  private config: WidgetConfig;
  private state: StateManager;
  private container: HTMLDivElement;
  private button: HTMLButtonElement;
  private panel: HTMLDivElement;
  private messagesContainer: HTMLDivElement;
  private inputArea: HTMLDivElement;

  private pollInterval: any = null;

  constructor(config: WidgetConfig) {
    this.config = config;
    this.state = new StateManager(config.tenant);

    // Create widget container
    this.container = document.createElement('div');
    this.container.id = 'agent-chat-widget';

    // Create components
    this.button = createButton(() => this.handleToggle());
    this.panel = createPanel(
      config.companyName ? `Chat with ${config.companyName}` : 'Chat with us',
      {
        onClose: () => this.handleClose(),
        onMinimize: () => this.handleMinimize(),
      }
    );
    this.messagesContainer = createMessagesContainer(config.welcomeMessage);
    this.inputArea = createInputArea({
      onSend: (message) => this.handleSend(message),
    });

    // Assemble widget
    const content = this.panel.querySelector('#acw-content');
    if (content) {
      content.appendChild(this.messagesContainer);
      content.appendChild(this.inputArea);
    }

    this.container.appendChild(this.button);
    this.container.appendChild(this.panel);

    // Subscribe to state changes
    this.state.subscribe((state) => {
      this.render(state);
      this.handlePollingLifecycle(state);
    });
  }

  /**
   * Mount widget to the DOM
   */
  mount(): void {
    // Inject styles
    const styleEl = document.createElement('style');
    styleEl.id = 'acw-styles';
    styleEl.textContent = getStyles(this.config);
    document.head.appendChild(styleEl);

    // Append widget to body
    document.body.appendChild(this.container);

    // Initial render
    this.render(this.state.getState());

    // Load initial transcript if session exists in localStorage
    this.initSession();
  }

  private async initSession(): Promise<void> {
    const key = `bchat_session_id:${this.config.tenant}`;
    const savedSessionId = localStorage.getItem(key);
    if (savedSessionId) {
      this.state.setSessionId(savedSessionId);
      await this.fetchTranscript(savedSessionId);
    }
  }

  private async fetchTranscript(sessionId: string): Promise<void> {
    try {
      const url = `${this.config.baseUrl}/api/v1/agent/${this.config.tenant}/chat/ext/transcript?session_id=${sessionId}`;
      const res = await fetch(url);
      if (res.ok) {
        const data = await res.json();
        if (data.messages) {
          const messages = data.messages.map((m: any) => ({
            role: m.role === 'user' ? 'user' : 'assistant',
            content: m.content,
            timestamp: new Date(m.timestamp),
          }));
          this.state.setMessages(messages);
        }
        if (data.bridge) {
          this.state.setBridge(data.bridge);
        } else {
          this.state.setBridge(null);
        }
      }
    } catch (e) {
      console.error('[AgentChatWidget] Failed to load transcript:', e);
    }
  }

  private startPolling(): void {
    if (this.pollInterval) return;
    this.pollInterval = setInterval(async () => {
      const state = this.state.getState();
      if (!state.sessionId || !state.isOpen || state.isMinimized) {
        this.stopPolling();
        return;
      }
      await this.fetchTranscript(state.sessionId);
    }, 3000);
  }

  private stopPolling(): void {
    if (this.pollInterval) {
      clearInterval(this.pollInterval);
      this.pollInterval = null;
    }
  }

  private handlePollingLifecycle(state: WidgetState): void {
    const isHandoffActive = state.bridge && (state.bridge.status === 'human_handoff_active' || state.bridge.status === 'human_handoff_queued');
    const shouldPoll = state.isOpen && !state.isMinimized && state.sessionId && isHandoffActive;

    if (shouldPoll) {
      this.startPolling();
    } else {
      this.stopPolling();
    }
  }

  /**
   * Unmount widget from the DOM
   */
  unmount(): void {
    this.stopPolling();
    const styleEl = document.getElementById('acw-styles');
    if (styleEl) {
      styleEl.remove();
    }
    this.container.remove();
  }

  /**
   * Open the chat panel
   */
  open(): void {
    this.state.setOpen(true);
    setTimeout(() => focusInput(this.inputArea), 100);
  }

  /**
   * Close the chat panel
   */
  close(): void {
    this.state.setOpen(false);
  }

  /**
   * Render widget based on current state
   */
  private render(state: WidgetState): void {
    updateButtonVisibility(this.button, state.isOpen);
    updatePanelOpen(this.panel, state.isOpen);
    updatePanelMinimized(this.panel, state.isMinimized);
    updateMessages(
      this.messagesContainer,
      state.messages,
      state.isLoading,
      state.error,
      this.config.welcomeMessage
    );
    updateInputDisabled(this.inputArea, state.isLoading);
  }

  /**
   * Handle toggle button click
   */
  private handleToggle(): void {
    this.state.toggleOpen();
    const currentState = this.state.getState();
    if (currentState.isOpen) {
      // Focus input when opening
      setTimeout(() => focusInput(this.inputArea), 100);
    }
  }

  /**
   * Handle close button click
   */
  private handleClose(): void {
    this.state.setOpen(false);
  }

  /**
   * Handle minimize button click
   */
  private handleMinimize(): void {
    this.state.toggleMinimized();
  }

  /**
   * Handle sending a message
   */
  private async handleSend(message: string): Promise<void> {
    const state = this.state.getState();
    if (state.isLoading) return;

    // Add user message
    this.state.addMessage({
      role: 'user',
      content: message,
      timestamp: new Date(),
    });

    // Clear any previous error
    this.state.setError(null);
    this.state.setLoading(true);
    const clientMessageId = this.state.getOrCreatePendingMessageID(message);

    try {
      const response = await sendMessage(
        this.config,
        message,
        state.sessionId,
        clientMessageId
      );
      this.state.acknowledgePendingMessage(message, clientMessageId);

      // Update session ID
      this.state.setSessionId(response.session_id);

      // Update bridge state
      this.state.setBridge(response.bridge || null);

      // Add assistant message
      this.state.addMessage({
        role: response.message.role === 'user' ? 'user' : 'assistant',
        content: response.message.content,
        timestamp: new Date(response.message.timestamp),
      });
    } catch (error) {
      if (error instanceof RateLimitError) {
        this.state.setError(error.message);
      } else if (error instanceof Error) {
        this.state.setError('Something went wrong. Please try again.');
        console.error('[AgentChatWidget] Error:', error.message);
      }
    } finally {
      this.state.setLoading(false);
    }
  }
}
