FROM node:26.13.1-trixie-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY cloudflare.config.ts worker.js ./
CMD ["npm", "run", "dev:r2", "--", "--ip", "0.0.0.0", "--port", "8788"]
