import { Icons } from '../styles/styles';

/**
 * Create the floating toggle button
 */
export function createButton(onClick: () => void): HTMLButtonElement {
  const button = document.createElement('button');
  button.id = 'acw-toggle';
  button.setAttribute('aria-label', 'Open chat');
  button.innerHTML = Icons.chat;
  button.addEventListener('click', onClick);
  return button;
}

/**
 * Update button visibility
 */
export function updateButtonVisibility(button: HTMLButtonElement, isOpen: boolean): void {
  if (isOpen) {
    button.classList.add('acw-hidden');
  } else {
    button.classList.remove('acw-hidden');
  }
}
