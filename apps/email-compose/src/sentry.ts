import * as Sentry from '@sentry/node';
import { nodeProfilingIntegration } from '@sentry/profiling-node';

export function initSentry() {
  const dsn = process.env.SENTRY_DSN;
  if (!dsn) return;

  Sentry.init({
    dsn,
    environment: process.env.SENTRY_ENV || process.env.NODE_ENV || 'development',
    tracesSampleRate: Number(process.env.SENTRY_TRACES_SAMPLE_RATE ?? '0.05'),
    profilesSampleRate: Number(process.env.SENTRY_PROFILES_SAMPLE_RATE ?? '0'),
    integrations: [nodeProfilingIntegration()],
    beforeSend(event) {
      // Basic scrub of headers/cookies/query
      if (event.request) {
        if (event.request.headers) {
          for (const k of Object.keys(event.request.headers)) {
            const lower = k.toLowerCase();
            if (['authorization', 'cookie', 'x-api-key'].includes(lower)) {
              event.request.headers[k] = '[REDACTED]';
            }
          }
        }
        if (event.request.cookies) {
          for (const k of Object.keys(event.request.cookies)) {
            event.request.cookies[k] = '[REDACTED]';
          }
        }
        if (event.request.query_string) {
          event.request.query_string = '[REDACTED]';
        }
        // Satisfy type: ensure Data is a string
        // @ts-ignore - Sentry types allow string or undefined
        event.request.data = '';
      }
      return event;
    },
  });
}
