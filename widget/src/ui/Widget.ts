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

  constructor(config: WidgetConfig) {
    this.config = config;
    this.state = new StateManager();

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
    this.state.subscribe((state) => this.render(state));
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
  }

  /**
   * Unmount widget from the DOM
   */
  unmount(): void {
    const styleEl = document.getElementById('acw-styles');
    if (styleEl) {
      styleEl.remove();
    }
    this.container.remove();
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

    try {
      const response = await sendMessage(
        this.config,
        message,
        state.sessionId
      );

      // Update session ID
      this.state.setSessionId(response.session_id);

      // Add assistant message
      this.state.addMessage({
        role: 'assistant',
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
