import type { Message } from '../core/types';
import { Icons } from '../styles/styles';

/**
 * Create the messages container
 */
export function createMessagesContainer(welcomeMessage: string): HTMLDivElement {
  const container = document.createElement('div');
  container.id = 'acw-messages';

  // Add empty state
  const emptyState = createEmptyState(welcomeMessage);
  container.appendChild(emptyState);

  return container;
}

/**
 * Create empty state element
 */
function createEmptyState(message: string): HTMLDivElement {
  const empty = document.createElement('div');
  empty.id = 'acw-empty';
  empty.innerHTML = `${Icons.chat}<p>${escapeHtml(message)}</p>`;
  return empty;
}

/**
 * Update messages display
 */
export function updateMessages(
  container: HTMLDivElement,
  messages: Message[],
  isLoading: boolean,
  error: string | null,
  welcomeMessage: string
): void {
  // Clear current content
  container.innerHTML = '';

  if (messages.length === 0 && !isLoading && !error) {
    // Show empty state
    container.appendChild(createEmptyState(welcomeMessage));
    return;
  }

  // Render messages
  messages.forEach((msg) => {
    container.appendChild(createMessageElement(msg));
  });

  // Show typing indicator
  if (isLoading) {
    container.appendChild(createTypingIndicator());
  }

  // Show error
  if (error) {
    container.appendChild(createErrorElement(error));
  }

  // Scroll to bottom
  container.scrollTop = container.scrollHeight;
}

/**
 * Create a message element
 */
function createMessageElement(message: Message): HTMLDivElement {
  const wrapper = document.createElement('div');
  wrapper.className = `acw-msg acw-msg-${message.role}`;

  const bubble = document.createElement('div');
  bubble.className = 'acw-msg-bubble';
  bubble.textContent = message.content;

  wrapper.appendChild(bubble);
  return wrapper;
}

/**
 * Create typing indicator
 */
function createTypingIndicator(): HTMLDivElement {
  const wrapper = document.createElement('div');
  wrapper.className = 'acw-typing';

  const bubble = document.createElement('div');
  bubble.className = 'acw-typing-bubble';

  const dots = document.createElement('div');
  dots.className = 'acw-typing-dots';
  dots.innerHTML = '<span></span><span></span><span></span>';

  bubble.appendChild(dots);
  wrapper.appendChild(bubble);
  return wrapper;
}

/**
 * Create error element
 */
function createErrorElement(error: string): HTMLDivElement {
  const el = document.createElement('div');
  el.className = 'acw-error';
  el.textContent = error;
  return el;
}

/**
 * Escape HTML to prevent XSS
 */
function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}
