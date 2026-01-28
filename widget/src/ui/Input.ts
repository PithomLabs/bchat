import { Icons } from '../styles/styles';

interface InputCallbacks {
  onSend: (message: string) => void;
}

/**
 * Create the input area with pill-style wrapper
 */
export function createInputArea(callbacks: InputCallbacks): HTMLDivElement {
  const container = document.createElement('div');
  container.id = 'acw-input-area';

  // Pill-style wrapper for input and button
  const wrapper = document.createElement('div');
  wrapper.id = 'acw-input-wrapper';

  const input = document.createElement('input');
  input.id = 'acw-input';
  input.type = 'text';
  input.placeholder = 'Type a message...';
  input.autocomplete = 'off';

  const sendBtn = document.createElement('button');
  sendBtn.id = 'acw-send';
  sendBtn.setAttribute('aria-label', 'Send message');
  sendBtn.innerHTML = Icons.send;

  // Handle send on click
  sendBtn.addEventListener('click', () => {
    const message = input.value.trim();
    if (message) {
      callbacks.onSend(message);
      input.value = '';
    }
  });

  // Handle send on Enter key
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      const message = input.value.trim();
      if (message) {
        callbacks.onSend(message);
        input.value = '';
      }
    }
  });

  wrapper.appendChild(input);
  wrapper.appendChild(sendBtn);
  container.appendChild(wrapper);

  return container;
}

/**
 * Update input disabled state
 */
export function updateInputDisabled(
  container: HTMLDivElement,
  isDisabled: boolean
): void {
  const input = container.querySelector('#acw-input') as HTMLInputElement | null;
  const sendBtn = container.querySelector('#acw-send') as HTMLButtonElement | null;

  if (input) {
    input.disabled = isDisabled;
  }
  if (sendBtn) {
    sendBtn.disabled = isDisabled;
  }
}

/**
 * Focus the input field
 */
export function focusInput(container: HTMLDivElement): void {
  const input = container.querySelector('#acw-input') as HTMLInputElement | null;
  if (input) {
    input.focus();
  }
}
