/**
 * Embeddable Chat Widget
 *
 * Usage (recommended - matches Agent Admin output):
 * <script src="https://your-server.com/widget/tenant-slug/embed.js"></script>
 * <script>
 *   AgentChatWidget.init({
 *     tenant: 'tenant-slug',
 *     baseUrl: 'https://your-server.com',
 *     color: '#0d9488',
 *     position: 'bottom-right',
 *     welcomeMessage: 'How can we help?',
 *     companyName: 'Your Company'
 *   });
 * </script>
 *
 * Alternative (legacy - auto-init):
 * <script>
 *   window.AgentChatConfig = { ... };
 * </script>
 * <script src="https://your-server.com/widget/embed.min.js"></script>
 */

import type { WidgetConfig } from './core/types';
import { DEFAULT_CONFIG } from './core/types';
import { Widget } from './ui/Widget';

// Declare global types
declare global {
  interface Window {
    AgentChatConfig?: Partial<WidgetConfig>;
    AgentChatWidget: typeof AgentChatWidgetAPI;
  }
}

/**
 * Parse configuration from script tag data attributes
 */
function getConfigFromScript(): Partial<WidgetConfig> {
  // Find our script tag
  const scripts = document.getElementsByTagName('script');
  let currentScript: HTMLScriptElement | null = null;

  for (let i = scripts.length - 1; i >= 0; i--) {
    const script = scripts[i];
    if (script.src && script.src.includes('embed')) {
      currentScript = script;
      break;
    }
  }

  if (!currentScript) {
    return {};
  }

  const config: Partial<WidgetConfig> = {};

  // Extract tenant from URL path: /widget/{tenant}/embed.js
  const urlMatch = currentScript.src.match(/\/widget\/([^/]+)\/embed/);
  if (urlMatch) {
    config.tenant = urlMatch[1];
    // Extract base URL
    const url = new URL(currentScript.src);
    config.baseUrl = url.origin;
  }

  // Parse data attributes
  const dataset = currentScript.dataset;

  if (dataset.tenant) config.tenant = dataset.tenant;
  if (dataset.baseUrl) config.baseUrl = dataset.baseUrl;
  if (dataset.companyName) config.companyName = dataset.companyName;
  if (dataset.color) config.color = dataset.color;
  if (dataset.position && (dataset.position === 'bottom-right' || dataset.position === 'bottom-left')) {
    config.position = dataset.position;
  }
  if (dataset.welcome) config.welcomeMessage = dataset.welcome;
  if (dataset.buttonSize) config.buttonSize = parseInt(dataset.buttonSize, 10);
  if (dataset.panelWidth) config.panelWidth = parseInt(dataset.panelWidth, 10);
  if (dataset.panelHeight) config.panelHeight = parseInt(dataset.panelHeight, 10);

  return config;
}

/**
 * Merge configurations with proper precedence:
 * 1. Default config (lowest priority)
 * 2. Global window.AgentChatConfig
 * 3. Script data attributes (highest priority)
 */
function mergeConfig(scriptConfig: Partial<WidgetConfig>): WidgetConfig {
  const globalConfig = window.AgentChatConfig || {};

  const merged: WidgetConfig = {
    baseUrl: scriptConfig.baseUrl || globalConfig.baseUrl || '',
    tenant: scriptConfig.tenant || globalConfig.tenant || '',
    companyName: scriptConfig.companyName || globalConfig.companyName,
    color: scriptConfig.color || globalConfig.color || DEFAULT_CONFIG.color,
    position: scriptConfig.position || globalConfig.position || DEFAULT_CONFIG.position,
    welcomeMessage: scriptConfig.welcomeMessage || globalConfig.welcomeMessage || DEFAULT_CONFIG.welcomeMessage,
    buttonSize: scriptConfig.buttonSize || globalConfig.buttonSize || DEFAULT_CONFIG.buttonSize,
    panelWidth: scriptConfig.panelWidth || globalConfig.panelWidth || DEFAULT_CONFIG.panelWidth,
    panelHeight: scriptConfig.panelHeight || globalConfig.panelHeight || DEFAULT_CONFIG.panelHeight,
  };

  return merged;
}

// Store widget instance
let widgetInstance: Widget | null = null;

/**
 * Initialize the widget with explicit config (recommended)
 */
function initWithConfig(userConfig: Partial<WidgetConfig>): void {
  if (widgetInstance) {
    console.warn('[AgentChatWidget] Widget already initialized');
    return;
  }

  const scriptConfig = getConfigFromScript();

  // User config takes precedence over script config
  const config: WidgetConfig = {
    baseUrl: userConfig.baseUrl || scriptConfig.baseUrl || '',
    tenant: userConfig.tenant || scriptConfig.tenant || '',
    companyName: userConfig.companyName || scriptConfig.companyName,
    color: userConfig.color || scriptConfig.color || DEFAULT_CONFIG.color,
    position: userConfig.position || scriptConfig.position || DEFAULT_CONFIG.position,
    welcomeMessage: userConfig.welcomeMessage || scriptConfig.welcomeMessage || DEFAULT_CONFIG.welcomeMessage,
    buttonSize: userConfig.buttonSize || scriptConfig.buttonSize || DEFAULT_CONFIG.buttonSize,
    panelWidth: userConfig.panelWidth || scriptConfig.panelWidth || DEFAULT_CONFIG.panelWidth,
    panelHeight: userConfig.panelHeight || scriptConfig.panelHeight || DEFAULT_CONFIG.panelHeight,
  };

  if (!config.baseUrl || !config.tenant) {
    console.error(
      '[AgentChatWidget] Missing required configuration. ' +
      'Ensure baseUrl and tenant are provided.'
    );
    return;
  }

  widgetInstance = new Widget(config);
  widgetInstance.mount();

  console.log('[AgentChatWidget] Initialized for tenant:', config.tenant);
}

/**
 * Auto-initialize from window.AgentChatConfig (legacy support)
 */
function autoInit(): void {
  if (widgetInstance) {
    return; // Already initialized via init()
  }

  // Only auto-init if AgentChatConfig is set
  if (!window.AgentChatConfig) {
    return; // Wait for explicit init() call
  }

  const scriptConfig = getConfigFromScript();
  const config = mergeConfig(scriptConfig);

  if (!config.baseUrl || !config.tenant) {
    // Don't log error - user might call init() later
    return;
  }

  widgetInstance = new Widget(config);
  widgetInstance.mount();

  console.log('[AgentChatWidget] Auto-initialized for tenant:', config.tenant);
}

/**
 * Public API object
 */
const AgentChatWidgetAPI = {
  init: (config: Partial<WidgetConfig>) => {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', () => initWithConfig(config));
    } else {
      initWithConfig(config);
    }
  },

  getInstance: () => widgetInstance,

  open: () => widgetInstance?.open(),

  close: () => widgetInstance?.close(),
};

// Export API globally
window.AgentChatWidget = AgentChatWidgetAPI;

// Auto-initialize when DOM is ready (legacy support for window.AgentChatConfig)
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', autoInit);
} else {
  autoInit();
}
