/**
 * iframe Entry Point
 *
 * This script is loaded inside an iframe and renders the widget
 * in full-panel mode (no floating button).
 *
 * Configuration is passed via query parameters:
 * /widget/{tenant}/iframe?color=%230d9488&welcome=Hello
 */

import type { WidgetConfig } from './core/types';
import { DEFAULT_CONFIG } from './core/types';
import { Widget } from './ui/Widget';

/**
 * Parse configuration from URL query parameters
 */
function getConfigFromUrl(): Partial<WidgetConfig> {
  const params = new URLSearchParams(window.location.search);
  const config: Partial<WidgetConfig> = {};

  // Extract tenant from URL path: /widget/{tenant}/iframe
  const pathMatch = window.location.pathname.match(/\/widget\/([^/]+)\/iframe/);
  if (pathMatch) {
    config.tenant = pathMatch[1];
    config.baseUrl = window.location.origin;
  }

  // Parse query parameters
  const tenant = params.get('tenant');
  if (tenant) config.tenant = tenant;

  const baseUrl = params.get('baseUrl');
  if (baseUrl) config.baseUrl = baseUrl;

  const companyName = params.get('companyName');
  if (companyName) config.companyName = companyName;

  const color = params.get('color');
  if (color) config.color = color;

  const position = params.get('position');
  if (position === 'bottom-right' || position === 'bottom-left') {
    config.position = position;
  }

  const welcome = params.get('welcome');
  if (welcome) config.welcomeMessage = welcome;

  return config;
}

/**
 * Build full config with defaults
 */
function buildConfig(partial: Partial<WidgetConfig>): WidgetConfig {
  return {
    baseUrl: partial.baseUrl || '',
    tenant: partial.tenant || '',
    companyName: partial.companyName,
    color: partial.color || DEFAULT_CONFIG.color,
    position: partial.position || DEFAULT_CONFIG.position,
    welcomeMessage: partial.welcomeMessage || DEFAULT_CONFIG.welcomeMessage,
    buttonSize: DEFAULT_CONFIG.buttonSize,
    panelWidth: DEFAULT_CONFIG.panelWidth,
    panelHeight: DEFAULT_CONFIG.panelHeight,
  };
}

/**
 * Initialize widget in iframe mode
 */
function init(): void {
  const urlConfig = getConfigFromUrl();
  const config = buildConfig(urlConfig);

  // Validate required config
  if (!config.baseUrl || !config.tenant) {
    document.body.innerHTML = `
      <div style="padding: 20px; color: #dc2626; font-family: system-ui;">
        <strong>Configuration Error</strong><br>
        Missing required tenant configuration.
      </div>
    `;
    console.error('[AgentChatWidget] Missing baseUrl or tenant in URL');
    return;
  }

  // Create and mount widget
  const widget = new Widget(config);
  widget.mount();

  // Auto-open in iframe mode
  // The widget state manager handles this, but we need to trigger it
  const toggleBtn = document.getElementById('acw-toggle');
  if (toggleBtn) {
    toggleBtn.click();
  }

  console.log('[AgentChatWidget iframe] Initialized for tenant:', config.tenant);
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
