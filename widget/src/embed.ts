/**
 * Embeddable Chat Widget
 *
 * Usage:
 * <script
 *   src="https://your-server.com/widget/tenant-slug/embed.js"
 *   data-position="bottom-right"
 *   data-color="#0d9488"
 *   data-welcome="How can we help?"
 * ></script>
 *
 * Or configure via global:
 * <script>
 *   window.AgentChatConfig = {
 *     baseUrl: 'https://your-server.com',
 *     tenant: 'tenant-slug',
 *     color: '#0d9488',
 *     position: 'bottom-right'
 *   };
 * </script>
 * <script src="https://your-server.com/widget/embed.min.js"></script>
 */

import type { WidgetConfig } from './core/types';
import { DEFAULT_CONFIG } from './core/types';
import { Widget } from './ui/Widget';

// Declare global config type
declare global {
  interface Window {
    AgentChatConfig?: Partial<WidgetConfig>;
    AgentChatWidget?: Widget;
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

/**
 * Initialize the widget
 */
function init(): void {
  // Don't initialize twice
  if (window.AgentChatWidget) {
    console.warn('[AgentChatWidget] Widget already initialized');
    return;
  }

  // Get configuration
  const scriptConfig = getConfigFromScript();
  const config = mergeConfig(scriptConfig);

  // Validate required config
  if (!config.baseUrl || !config.tenant) {
    console.error(
      '[AgentChatWidget] Missing required configuration. ' +
      'Ensure baseUrl and tenant are provided via data attributes or window.AgentChatConfig'
    );
    return;
  }

  // Create and mount widget
  const widget = new Widget(config);
  widget.mount();

  // Store reference globally for debugging/testing
  window.AgentChatWidget = widget;

  console.log('[AgentChatWidget] Initialized for tenant:', config.tenant);
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
