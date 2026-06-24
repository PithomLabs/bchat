import type { WidgetConfig } from '../core/types';
import interFontUrl from '../assets/fonts/inter-latin-wght-normal.woff2?inline';

/**
 * Generate CSS styles for the widget
 * Premium chat widget design inspired by Intercom
 */
export function getStyles(config: WidgetConfig): string {
  const { color, position, buttonSize, panelWidth, panelHeight } = config;
  const positionRight = position === 'bottom-right';

  return `
    @font-face {
      font-family: 'Inter';
      src: url('${interFontUrl}') format('woff2');
      font-style: normal;
      font-weight: 100 900;
      font-display: swap;
    }

    /* Widget Reset */
    #agent-chat-widget,
    #agent-chat-widget * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
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
      background: ${color};
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
      bottom: 20px;
      ${positionRight ? 'right: 20px;' : 'left: 20px;'}
      width: min(700px, 90vw);
      height: min(600px, 90vh);
      background: #18181b;
      border: 1px solid #27272a;
      border-radius: 12px;
      box-shadow: 0 18px 60px rgba(0, 0, 0, 0.5);
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
      background: rgba(24, 24, 27, 0.96);
      padding: 14px 16px;
      color: #e4e4e7;
      border-bottom: 1px solid #27272a;
      backdrop-filter: blur(12px);
    }

    #acw-header-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    #acw-header-title > div:first-child {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    #acw-header-title svg {
      width: 24px;
      height: 24px;
      fill: ${color};
    }

    #acw-header-title span {
      font-size: 15px;
      font-weight: 600;
      letter-spacing: -0.015em;
    }

    #acw-header-controls {
      display: flex;
      gap: 4px;
    }

    #acw-header-controls button {
      background: #27272a;
      border: 1px solid transparent;
      width: 32px;
      height: 32px;
      cursor: pointer;
      border-radius: 9px;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.2s ease;
    }

    #acw-header-controls button:hover {
      background: #3f3f46;
      border-color: #52525b;
    }

    #acw-header-controls svg {
      width: 16px;
      height: 16px;
      fill: #a1a1aa;
    }

    /* Content Area */
    #acw-content {
      flex: 1;
      display: flex;
      flex-direction: column;
      overflow: hidden;
      background: #09090b;
      position: relative;
      z-index: 1;
    }

    /* Messages */
    #acw-messages {
      flex: 1;
      overflow-y: auto;
      padding: 0;
      display: flex;
      flex-direction: column;
      gap: 12px;
      scroll-behavior: smooth;
    }

    #acw-messages::-webkit-scrollbar {
      width: 5px;
    }

    #acw-messages::-webkit-scrollbar-track {
      background: transparent;
    }

    #acw-messages::-webkit-scrollbar-thumb {
      background: rgba(255, 255, 255, 0.15);
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
      color: #a1a1aa;
      line-height: 1.6;
      max-width: 240px;
    }

    /* Message Row */
    .acw-msg {
      display: flex;
      width: 100%;
      animation: acw-fade-in 0.3s ease;
    }

    @keyframes acw-fade-in {
      from { opacity: 0; transform: translateY(8px); }
      to { opacity: 1; transform: translateY(0); }
    }

    .acw-msg-user {
      justify-content: flex-end;
      margin-left: 32px;
    }

    .acw-msg-assistant {
      justify-content: flex-start;
      margin-right: 32px;
    }

    /* Message Bubble */
    .acw-msg-bubble {
      min-width: 120px;
      padding: 14px 16px;
      border-radius: 8px;
      font-size: 14px;
      line-height: 1.75;
      text-align: left;
      word-wrap: break-word;
      white-space: pre-wrap;
    }

    /* Customer (User) Bubble Specifics */
    .acw-msg-user .acw-msg-bubble {
      background-color: #1e3a5f;
      border: 1px solid #2563eb;
      color: #e4e4e7;
    }

    /* Agent (Assistant) Bubble Specifics */
    .acw-msg-assistant .acw-msg-bubble {
      background-color: #3f3f46;
      border: 1px solid #52525b;
      color: #e4e4e7;
    }

    /* Header inside bubble */
    .acw-msg-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 4px;
    }

    /* Role label */
    .acw-msg-role {
      font-size: 12px;
      font-weight: 500;
    }

    .acw-msg-user .acw-msg-role {
      color: #60a5fa;
    }

    .acw-msg-assistant .acw-msg-role {
      color: #71717a;
    }

    /* Timestamp */
    .acw-msg-time {
      font-size: 12px;
      color: #71717a;
    }

    /* Content text */
    .acw-msg-content {
      font-size: 14px;
      color: #e4e4e7;
      line-height: 1.6;
      text-align: left;
      white-space: pre-wrap;
    }

    /* Typing Indicator */
    .acw-typing {
      display: flex;
      justify-content: flex-start;
      margin-bottom: 16px;
    }

    .acw-typing-bubble {
      background: #18181b;
      padding: 13px 17px;
      border: 1px solid #27272a;
      border-radius: 12px;
      box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
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
      background: #450a0a;
      color: #fca5a5;
      padding: 12px 16px;
      border-radius: 12px;
      font-size: 13px;
      margin-bottom: 16px;
      border: 1px solid #7f1d1d;
    }

    /* Input Area */
    #acw-input-area {
      background: #18181b;
      padding: 12px 16px 16px;
      border-top: 1px solid #27272a;
    }

    #acw-input-wrapper {
      display: flex;
      align-items: flex-end;
      gap: 8px;
      background: #3f3f46;
      border: 1px solid #52525b;
      border-radius: 8px;
      padding: 14px 16px;
      transition: border-color 0.2s ease;
    }

    #acw-input-wrapper:focus-within {
      border-color: ${color};
    }

    #acw-input {
      flex: 1;
      border: none;
      background: transparent;
      font-size: 14px;
      line-height: 1.75;
      resize: none;
      outline: none;
      min-height: 24px;
      max-height: 100px;
      padding: 0;
      color: #e4e4e7;
    }

    #acw-input::placeholder {
      color: #71717a;
    }

    #acw-input:disabled {
      cursor: not-allowed;
      opacity: 0.6;
    }

    #acw-send {
      width: 32px;
      height: 32px;
      border: none;
      background: ${color};
      border-radius: 6px;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: opacity 0.15s ease;
      flex-shrink: 0;
    }

    #acw-send:hover:not(:disabled) {
      opacity: 0.9;
    }

    #acw-send:active:not(:disabled) {
      opacity: 0.75;
    }

    #acw-send:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    #acw-send svg {
      width: 16px;
      height: 16px;
      fill: white;
    }

    /* Mobile */
    @media (max-width: 480px) {
      #acw-panel {
        width: calc(100vw - 16px);
        height: min(600px, calc(100dvh - 60px));
        bottom: 8px;
        ${positionRight ? 'right: 8px;' : 'left: 8px;'}
        border-radius: 12px;
      }

      #acw-toggle {
        bottom: 16px;
        ${positionRight ? 'right: 16px;' : 'left: 16px;'}
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

function hexToRgba(hex: string, alpha: number): string {
  const normalized = hex.replace('#', '');
  const value = normalized.length === 3
    ? normalized.split('').map((char) => char + char).join('')
    : normalized;
  const parsed = parseInt(value, 16);
  const red = (parsed >> 16) & 255;
  const green = (parsed >> 8) & 255;
  const blue = parsed & 255;
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
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
