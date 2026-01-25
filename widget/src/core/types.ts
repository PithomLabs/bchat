/**
 * Widget configuration options
 */
export interface WidgetConfig {
  /** Base URL of the chat server */
  baseUrl: string;
  /** Tenant slug identifier */
  tenant: string;
  /** Company name to display in header */
  companyName?: string;
  /** Primary color for branding (hex) */
  color: string;
  /** Widget position on screen */
  position: 'bottom-right' | 'bottom-left';
  /** Welcome message shown when chat is empty */
  welcomeMessage: string;
  /** Toggle button size in pixels */
  buttonSize: number;
  /** Chat panel width in pixels */
  panelWidth: number;
  /** Chat panel height in pixels */
  panelHeight: number;
}

/**
 * Default widget configuration
 */
export const DEFAULT_CONFIG: Omit<WidgetConfig, 'baseUrl' | 'tenant'> = {
  color: '#0d9488',
  position: 'bottom-right',
  welcomeMessage: 'How can we help you today?',
  buttonSize: 56,
  panelWidth: 350,
  panelHeight: 500,
};

/**
 * Chat message
 */
export interface Message {
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
}

/**
 * Widget state
 */
export interface WidgetState {
  isOpen: boolean;
  isMinimized: boolean;
  isLoading: boolean;
  messages: Message[];
  sessionId: string | null;
  error: string | null;
}

/**
 * Initial widget state
 */
export const INITIAL_STATE: WidgetState = {
  isOpen: false,
  isMinimized: false,
  isLoading: false,
  messages: [],
  sessionId: null,
  error: null,
};

/**
 * API response for chat messages
 */
export interface ChatResponse {
  session_id: string;
  message: {
    role: 'assistant';
    content: string;
    timestamp: string;
  };
  metadata?: {
    intent?: string;
    confidence?: number;
  };
}

/**
 * API error response
 */
export interface ErrorResponse {
  error?: string;
  message?: string;
  retry_after?: number;
}
