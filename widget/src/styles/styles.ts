import type { WidgetConfig } from '../core/types';

/**
 * Generate CSS styles for the widget
 * All styles are scoped to #acw- prefixed IDs to avoid conflicts
 */
export function getStyles(config: WidgetConfig): string {
  const { color, position, buttonSize, panelWidth, panelHeight } = config;
  const positionRight = position === 'bottom-right';

  return `
    /* Widget Reset - Scoped to widget container */
    #agent-chat-widget,
    #agent-chat-widget * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      font-size: 14px;
      line-height: 1.5;
    }

    /* Toggle Button */
    #acw-toggle {
      position: fixed;
      bottom: 20px;
      ${positionRight ? 'right: 20px;' : 'left: 20px;'}
      width: ${buttonSize}px;
      height: ${buttonSize}px;
      border-radius: 50%;
      background: ${color};
      border: none;
      cursor: pointer;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
      display: flex;
      align-items: center;
      justify-content: center;
      transition: transform 0.2s ease, box-shadow 0.2s ease;
      z-index: 9998;
    }

    #acw-toggle:hover {
      transform: scale(1.1);
      box-shadow: 0 6px 16px rgba(0, 0, 0, 0.2);
    }

    #acw-toggle:focus {
      outline: none;
      box-shadow: 0 0 0 3px rgba(13, 148, 136, 0.3);
    }

    #acw-toggle svg {
      width: 24px;
      height: 24px;
      fill: white;
    }

    #acw-toggle.acw-hidden {
      display: none;
    }

    /* Chat Panel */
    #acw-panel {
      position: fixed;
      bottom: 20px;
      ${positionRight ? 'right: 20px;' : 'left: 20px;'}
      width: ${panelWidth}px;
      height: ${panelHeight}px;
      background: white;
      border-radius: 12px;
      box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
      display: none;
      flex-direction: column;
      overflow: hidden;
      z-index: 9999;
    }

    #acw-panel.acw-open {
      display: flex;
    }

    #acw-panel.acw-minimized {
      height: auto;
    }

    #acw-panel.acw-minimized #acw-content {
      display: none;
    }

    /* Header */
    #acw-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 16px;
      background: ${color};
      color: white;
    }

    #acw-header-title {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 500;
    }

    #acw-header-title svg {
      width: 20px;
      height: 20px;
      fill: white;
    }

    #acw-header-controls {
      display: flex;
      gap: 4px;
    }

    #acw-header-controls button {
      background: transparent;
      border: none;
      padding: 4px;
      cursor: pointer;
      border-radius: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.2s ease;
    }

    #acw-header-controls button:hover {
      background: rgba(255, 255, 255, 0.2);
    }

    #acw-header-controls svg {
      width: 16px;
      height: 16px;
      fill: white;
    }

    /* Content Area */
    #acw-content {
      flex: 1;
      display: flex;
      flex-direction: column;
      overflow: hidden;
    }

    /* Messages */
    #acw-messages {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
      background: #f9fafb;
    }

    #acw-messages::-webkit-scrollbar {
      width: 6px;
    }

    #acw-messages::-webkit-scrollbar-track {
      background: transparent;
    }

    #acw-messages::-webkit-scrollbar-thumb {
      background: #d1d5db;
      border-radius: 3px;
    }

    /* Empty State */
    #acw-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      color: #6b7280;
      text-align: center;
      padding: 20px;
    }

    #acw-empty svg {
      width: 40px;
      height: 40px;
      fill: #d1d5db;
      margin-bottom: 12px;
    }

    /* Message Bubbles */
    .acw-msg {
      margin-bottom: 12px;
      display: flex;
    }

    .acw-msg-user {
      justify-content: flex-end;
    }

    .acw-msg-assistant {
      justify-content: flex-start;
    }

    .acw-msg-bubble {
      max-width: 80%;
      padding: 14px 20px;
      border-radius: 12px;
      word-wrap: break-word;
      white-space: pre-wrap;
    }

    .acw-msg-user .acw-msg-bubble {
      background: ${color};
      color: white;
      border-bottom-right-radius: 4px;
    }

    .acw-msg-assistant .acw-msg-bubble {
      background: white;
      color: #374151;
      border: 1px solid #e5e7eb;
      border-bottom-left-radius: 4px;
    }

    /* Typing Indicator */
    .acw-typing {
      display: flex;
      justify-content: flex-start;
      margin-bottom: 12px;
    }

    .acw-typing-bubble {
      background: white;
      border: 1px solid #e5e7eb;
      padding: 10px 14px;
      border-radius: 12px;
      border-bottom-left-radius: 4px;
      color: #6b7280;
    }

    .acw-typing-dots {
      display: inline-flex;
      gap: 4px;
    }

    .acw-typing-dots span {
      width: 6px;
      height: 6px;
      background: #9ca3af;
      border-radius: 50%;
      animation: acw-typing-bounce 1.4s ease-in-out infinite;
    }

    .acw-typing-dots span:nth-child(2) {
      animation-delay: 0.2s;
    }

    .acw-typing-dots span:nth-child(3) {
      animation-delay: 0.4s;
    }

    @keyframes acw-typing-bounce {
      0%, 60%, 100% {
        transform: translateY(0);
      }
      30% {
        transform: translateY(-4px);
      }
    }

    /* Error Message */
    .acw-error {
      text-align: center;
      color: #dc2626;
      padding: 8px;
      font-size: 13px;
    }

    /* Input Area */
    #acw-input-area {
      display: flex;
      gap: 8px;
      padding: 12px 16px;
      border-top: 1px solid #e5e7eb;
      background: white;
    }

    #acw-input {
      flex: 1;
      padding: 10px 14px;
      border: 1px solid #d1d5db;
      border-radius: 8px;
      font-size: 14px;
      resize: none;
      outline: none;
      transition: border-color 0.2s ease;
    }

    #acw-input:focus {
      border-color: ${color};
    }

    #acw-input::placeholder {
      color: #9ca3af;
    }

    #acw-input:disabled {
      background: #f3f4f6;
      cursor: not-allowed;
    }

    #acw-send {
      padding: 10px 14px;
      background: ${color};
      color: white;
      border: none;
      border-radius: 8px;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: opacity 0.2s ease;
    }

    #acw-send:hover:not(:disabled) {
      opacity: 0.9;
    }

    #acw-send:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    #acw-send svg {
      width: 18px;
      height: 18px;
      fill: white;
    }

    /* Mobile Responsive */
    @media (max-width: 480px) {
      #acw-panel {
        width: calc(100vw - 24px);
        height: 60vh;
        bottom: 12px;
        ${positionRight ? 'right: 12px;' : 'left: 12px;'}
      }

      #acw-toggle {
        bottom: 12px;
        ${positionRight ? 'right: 12px;' : 'left: 12px;'}
      }
    }
  `;
}

/**
 * SVG Icons as strings
 */
export const Icons = {
  chat: `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h10v2H7zm0-3h10v2H7zm0 6h7v2H7z"/></svg>`,
  close: `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>`,
  minimize: `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M19 13H5v-2h14v2z"/></svg>`,
  send: `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>`,
};
