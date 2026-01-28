import type { WidgetConfig } from '../core/types';

/**
 * Generate CSS styles for the widget
 * Premium chat widget design inspired by Intercom
 */
export function getStyles(config: WidgetConfig): string {
  const { color, position, buttonSize, panelWidth, panelHeight } = config;
  const positionRight = position === 'bottom-right';

  return `
    /* Widget Reset */
    #agent-chat-widget,
    #agent-chat-widget * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
    }

    /* Launcher Button */
    #acw-toggle {
      position: fixed;
      bottom: 20px;
      ${positionRight ? 'right: 20px;' : 'left: 20px;'}
      width: ${buttonSize}px;
      height: ${buttonSize}px;
      border-radius: 50%;
      background: linear-gradient(135deg, ${color} 0%, ${shadeColor(color, -15)} 100%);
      border: none;
      cursor: pointer;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15), 0 0 0 0 ${color}40;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: transform 0.2s ease, box-shadow 0.3s ease;
      z-index: 9998;
    }

    #acw-toggle:hover {
      transform: scale(1.1);
      box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2), 0 0 0 4px ${color}20;
    }

    #acw-toggle:active {
      transform: scale(1.05);
    }

    #acw-toggle svg {
      width: 28px;
      height: 28px;
      fill: white;
      filter: drop-shadow(0 1px 1px rgba(0, 0, 0, 0.1));
    }

    #acw-toggle.acw-hidden {
      display: none;
    }

    /* Chat Panel */
    #acw-panel {
      position: fixed;
      bottom: 90px;
      ${positionRight ? 'right: 20px;' : 'left: 20px;'}
      width: ${panelWidth}px;
      height: ${panelHeight}px;
      background: #ffffff;
      border-radius: 16px;
      box-shadow: 0 5px 40px rgba(0, 0, 0, 0.16);
      display: none;
      flex-direction: column;
      overflow: hidden;
      z-index: 9999;
      transform-origin: bottom ${positionRight ? 'right' : 'left'};
      animation: acw-slide-up 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes acw-slide-up {
      from {
        opacity: 0;
        transform: translateY(20px) scale(0.95);
      }
      to {
        opacity: 1;
        transform: translateY(0) scale(1);
      }
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
      background: linear-gradient(135deg, ${color} 0%, ${shadeColor(color, -20)} 100%);
      padding: 20px 20px 24px;
      color: white;
      position: relative;
    }

    #acw-header::after {
      content: '';
      position: absolute;
      bottom: -12px;
      left: 0;
      right: 0;
      height: 24px;
      background: linear-gradient(135deg, ${color} 0%, ${shadeColor(color, -20)} 100%);
      border-radius: 0 0 50% 50% / 0 0 100% 100%;
    }

    #acw-header-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    #acw-header-title > div:first-child {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    #acw-header-title svg {
      width: 24px;
      height: 24px;
      fill: white;
    }

    #acw-header-title span {
      font-size: 16px;
      font-weight: 600;
      letter-spacing: -0.2px;
    }

    #acw-header-controls {
      display: flex;
      gap: 4px;
    }

    #acw-header-controls button {
      background: rgba(255, 255, 255, 0.2);
      backdrop-filter: blur(4px);
      border: none;
      width: 32px;
      height: 32px;
      cursor: pointer;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.2s ease;
    }

    #acw-header-controls button:hover {
      background: rgba(255, 255, 255, 0.3);
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
      background: #f6f6f6;
      position: relative;
      z-index: 1;
      margin-top: 12px;
    }

    /* Messages */
    #acw-messages {
      flex: 1;
      overflow-y: auto;
      padding: 20px 16px;
      scroll-behavior: smooth;
    }

    #acw-messages::-webkit-scrollbar {
      width: 5px;
    }

    #acw-messages::-webkit-scrollbar-track {
      background: transparent;
    }

    #acw-messages::-webkit-scrollbar-thumb {
      background: rgba(0, 0, 0, 0.15);
      border-radius: 10px;
    }

    /* Empty State */
    #acw-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      padding: 40px 24px;
      text-align: center;
    }

    #acw-empty svg {
      width: 56px;
      height: 56px;
      fill: ${color};
      opacity: 0.3;
      margin-bottom: 16px;
    }

    #acw-empty p {
      font-size: 15px;
      color: #65676b;
      line-height: 1.5;
      max-width: 240px;
    }

    /* Message Row */
    .acw-msg {
      display: flex;
      margin-bottom: 24px;
      animation: acw-fade-in 0.3s ease;
    }

    @keyframes acw-fade-in {
      from { opacity: 0; transform: translateY(8px); }
      to { opacity: 1; transform: translateY(0); }
    }

    .acw-msg-user {
      justify-content: flex-end;
    }

    .acw-msg-assistant {
      justify-content: flex-start;
    }

    /* Message Text - Plain, no bubble */
    .acw-msg-bubble {
      max-width: 90%;
      padding: 8px 0;
      font-size: 15px;
      line-height: 1.7;
      word-wrap: break-word;
      white-space: pre-wrap;
      color: #1c1e21;
    }

    /* Timestamp */
    .acw-msg-time {
      font-size: 11px;
      margin-top: 8px;
      display: block;
      opacity: 0.7;
    }

    .acw-msg-user .acw-msg-time {
      color: rgba(255, 255, 255, 0.85);
    }

    .acw-msg-assistant .acw-msg-time {
      color: #65676b;
    }

    /* Typing Indicator */
    .acw-typing {
      display: flex;
      justify-content: flex-start;
      margin-bottom: 16px;
    }

    .acw-typing-bubble {
      background: #ffffff;
      padding: 14px 18px;
      border-radius: 18px 18px 18px 4px;
      box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
    }

    .acw-typing-dots {
      display: flex;
      gap: 4px;
      align-items: center;
    }

    .acw-typing-dots span {
      width: 8px;
      height: 8px;
      background: ${color};
      border-radius: 50%;
      animation: acw-bounce 1.4s ease-in-out infinite;
    }

    .acw-typing-dots span:nth-child(2) {
      animation-delay: 0.2s;
    }

    .acw-typing-dots span:nth-child(3) {
      animation-delay: 0.4s;
    }

    @keyframes acw-bounce {
      0%, 60%, 100% {
        transform: translateY(0);
        opacity: 0.4;
      }
      30% {
        transform: translateY(-4px);
        opacity: 1;
      }
    }

    /* Error */
    .acw-error {
      background: #ffebe9;
      color: #cf222e;
      padding: 12px 16px;
      border-radius: 12px;
      font-size: 13px;
      margin-bottom: 16px;
      border: 1px solid #ffcecb;
    }

    /* Input Area */
    #acw-input-area {
      background: #ffffff;
      padding: 16px;
      border-top: 1px solid rgba(0, 0, 0, 0.06);
    }

    #acw-input-wrapper {
      display: flex;
      align-items: flex-end;
      gap: 10px;
      background: #f0f2f5;
      border-radius: 24px;
      padding: 6px 6px 6px 16px;
      transition: box-shadow 0.2s ease, background 0.2s ease;
    }

    #acw-input-wrapper:focus-within {
      background: #ffffff;
      box-shadow: 0 0 0 2px ${color}40;
    }

    #acw-input {
      flex: 1;
      border: none;
      background: transparent;
      font-size: 14px;
      line-height: 1.4;
      resize: none;
      outline: none;
      min-height: 24px;
      max-height: 100px;
      padding: 8px 0;
      color: #1c1e21;
    }

    #acw-input::placeholder {
      color: #8a8d91;
    }

    #acw-input:disabled {
      cursor: not-allowed;
      opacity: 0.6;
    }

    #acw-send {
      width: 36px;
      height: 36px;
      border: none;
      background: ${color};
      border-radius: 50%;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: transform 0.15s ease, opacity 0.15s ease;
      flex-shrink: 0;
    }

    #acw-send:hover:not(:disabled) {
      transform: scale(1.08);
    }

    #acw-send:active:not(:disabled) {
      transform: scale(0.95);
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

    /* Mobile */
    @media (max-width: 480px) {
      #acw-panel {
        width: calc(100vw - 16px);
        height: calc(100vh - 120px);
        max-height: 600px;
        bottom: 80px;
        ${positionRight ? 'right: 8px;' : 'left: 8px;'}
        border-radius: 20px;
      }

      #acw-toggle {
        bottom: 16px;
        ${positionRight ? 'right: 16px;' : 'left: 16px;'}
      }
    }

    /* Dark Mode */
    @media (prefers-color-scheme: dark) {
      #acw-panel {
        background: #242526;
        box-shadow: 0 5px 40px rgba(0, 0, 0, 0.4);
      }

      #acw-content {
        background: #18191a;
      }

      #acw-empty p {
        color: #b0b3b8;
      }

      .acw-msg-bubble {
        color: #ffffff;
      }

      .acw-msg-assistant .acw-msg-time {
        color: #b0b3b8;
      }

      .acw-msg-user .acw-msg-time {
        color: #b0b3b8;
      }

      .acw-typing-bubble {
        background: #3a3b3c;
      }

      .acw-error {
        background: #4a1c1c;
        border-color: #6b2c2c;
        color: #f97583;
      }

      #acw-input-area {
        background: #242526;
        border-top-color: rgba(255, 255, 255, 0.1);
      }

      #acw-input-wrapper {
        background: #3a3b3c;
      }

      #acw-input-wrapper:focus-within {
        background: #4e4f50;
      }

      #acw-input {
        color: #e4e6eb;
      }

      #acw-input::placeholder {
        color: #b0b3b8;
      }

      #acw-messages::-webkit-scrollbar-thumb {
        background: rgba(255, 255, 255, 0.2);
      }
    }
  `;
}

/**
 * Shade a hex color (positive = lighter, negative = darker)
 */
function shadeColor(hex: string, percent: number): string {
  const num = parseInt(hex.replace('#', ''), 16);
  const amt = Math.round(2.55 * percent);
  const R = Math.max(0, Math.min(255, (num >> 16) + amt));
  const G = Math.max(0, Math.min(255, ((num >> 8) & 0x00ff) + amt));
  const B = Math.max(0, Math.min(255, (num & 0x0000ff) + amt));
  return '#' + (0x1000000 + R * 0x10000 + G * 0x100 + B).toString(16).slice(1);
}

/**
 * SVG Icons
 */
export const Icons = {
  chat: `<svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.2L4 17.2V4h16v12z"/></svg>`,
  close: `<svg viewBox="0 0 24 24"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>`,
  minimize: `<svg viewBox="0 0 24 24"><path d="M19 13H5v-2h14v2z"/></svg>`,
  send: `<svg viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>`,
  bot: `<svg viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-4-8c.79 0 1.5-.71 1.5-1.5S8.79 9 8 9s-1.5.71-1.5 1.5S7.21 12 8 12zm8 0c.79 0 1.5-.71 1.5-1.5S16.79 9 16 9s-1.5.71-1.5 1.5.71 1.5 1.5 1.5zm-4 4c2.21 0 4-1.34 4-3h-8c0 1.66 1.79 3 4 3z"/></svg>`,
};
