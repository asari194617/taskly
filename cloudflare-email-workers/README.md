# Cloudflare Email Worker (Inbound Forwarder)

This Worker receives emails from Cloudflare Email Routing (Email Workers) and forwards them to Taskly's server-side endpoint with a header secret.

## Deploy

- Create a new Email Worker in Cloudflare (Email > Email Routing > Workers)
- Use `email-worker.js` as the script
- Bind these environment variables:
  - `EMAIL_INBOUND_ENDPOINT` – e.g. `https://your.server/integrations/email/inbound`
  - `EMAIL_INTEGRATION_SECRET` – must match the server's `EMAIL_INTEGRATION_SECRET`
- Configure Email Routing rules to send target messages to this Worker.

## Notes

- The Worker forwards the raw RFC822 email as base64 in `raw`. Parsing of attachments (DMARC XML, ZIP, etc.) is handled server-side.
- `text` and `html` bodies are not extracted by the Worker to keep it lightweight; server can extract from `raw` if needed.
- The server endpoint returns HTTP 202 on success and sends Discord embeds for DMARC/bounce/generic events.
