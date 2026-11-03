# syntax=docker/dockerfile:1.6

FROM node:26.13.0-trixie AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# Use BuildKit cache to speed up installs
RUN --mount=type=cache,target=/root/.npm \
	npm ci --no-audit --no-fund --ignore-scripts

FROM deps AS build
WORKDIR /app
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# The slim runtime omits the vulnerable distro libbrotli/curl stack. Node
# 26.13.0 carries its own CVE-2026-111497 fix.
FROM node:26.13.0-trixie-slim AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN test "$(node --version)" = "v26.13.0" \
	&& ! command -v curl \
	&& ! dpkg-query -W libbrotli1 >/dev/null 2>&1
# Install production deps only
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
	npm ci --omit=dev --no-audit --no-fund --ignore-scripts
COPY --from=build /app/dist ./dist
EXPOSE 50051 50052
CMD ["node", "dist/index.js"]
