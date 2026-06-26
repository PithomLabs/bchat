import type { ChatResponse, ErrorResponse, WidgetConfig } from './types';

/**
 * Send a chat message to the external chat API
 */
export async function sendMessage(
  config: WidgetConfig,
  message: string,
  sessionId: string | null,
  clientMessageId: string
): Promise<ChatResponse> {
  const url = `${config.baseUrl}/api/v1/agent/${config.tenant}/chat/ext`;

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      session_id: sessionId,
      message: message,
      client_message_id: clientMessageId,
    }),
  });

  if (response.status === 429) {
    const errorData: ErrorResponse = await response.json().catch(() => ({}));
    const retryAfter = errorData.retry_after || 60;
    throw new RateLimitError(
      `Too many messages. Please try again in ${retryAfter} seconds.`,
      retryAfter
    );
  }

  if (!response.ok) {
    const errorData: ErrorResponse = await response.json().catch(() => ({}));
    throw new ApiError(
      errorData.message || errorData.error || 'Failed to send message',
      response.status
    );
  }

  return response.json();
}

/**
 * Custom error for rate limiting
 */
export class RateLimitError extends Error {
  constructor(
    message: string,
    public retryAfter: number
  ) {
    super(message);
    this.name = 'RateLimitError';
  }
}

/**
 * Custom error for API errors
 */
export class ApiError extends Error {
  constructor(
    message: string,
    public statusCode: number
  ) {
    super(message);
    this.name = 'ApiError';
  }
}
