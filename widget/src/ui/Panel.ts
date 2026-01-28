import { Icons } from '../styles/styles';

interface PanelCallbacks {
  onClose: () => void;
  onMinimize: () => void;
}

/**
 * Create the chat panel container
 */
export function createPanel(
  title: string,
  callbacks: PanelCallbacks
): HTMLDivElement {
  const panel = document.createElement('div');
  panel.id = 'acw-panel';

  // Header
  const header = document.createElement('div');
  header.id = 'acw-header';

  const headerTitle = document.createElement('div');
  headerTitle.id = 'acw-header-title';

  // Title content wrapper (for flex layout)
  const titleContent = document.createElement('div');
  titleContent.innerHTML = `${Icons.bot}<span>${escapeHtml(title)}</span>`;

  // Controls
  const headerControls = document.createElement('div');
  headerControls.id = 'acw-header-controls';

  const minimizeBtn = document.createElement('button');
  minimizeBtn.id = 'acw-minimize';
  minimizeBtn.setAttribute('aria-label', 'Minimize chat');
  minimizeBtn.innerHTML = Icons.minimize;
  minimizeBtn.addEventListener('click', callbacks.onMinimize);

  const closeBtn = document.createElement('button');
  closeBtn.id = 'acw-close';
  closeBtn.setAttribute('aria-label', 'Close chat');
  closeBtn.innerHTML = Icons.close;
  closeBtn.addEventListener('click', callbacks.onClose);

  headerControls.appendChild(minimizeBtn);
  headerControls.appendChild(closeBtn);

  headerTitle.appendChild(titleContent);
  headerTitle.appendChild(headerControls);
  header.appendChild(headerTitle);

  // Content container
  const content = document.createElement('div');
  content.id = 'acw-content';

  panel.appendChild(header);
  panel.appendChild(content);

  return panel;
}

/**
 * Update panel open state
 */
export function updatePanelOpen(panel: HTMLDivElement, isOpen: boolean): void {
  if (isOpen) {
    panel.classList.add('acw-open');
  } else {
    panel.classList.remove('acw-open');
  }
}

/**
 * Update panel minimized state
 */
export function updatePanelMinimized(panel: HTMLDivElement, isMinimized: boolean): void {
  if (isMinimized) {
    panel.classList.add('acw-minimized');
  } else {
    panel.classList.remove('acw-minimized');
  }
}

/**
 * Escape HTML to prevent XSS
 */
function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}
