FROM node:26.13.0-trixie AS deps
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* ./
RUN npm ci --legacy-peer-deps --no-audit --no-fund

FROM deps AS build
WORKDIR /app
COPY tsconfig.json ./
COPY src ./src
RUN npx tsc -p .

# The slim runtime omits the vulnerable distro libbrotli/curl stack. Node
# 26.12.1 carries its own CVE-2026-111497 fix.
FROM node:26.13.0-trixie-slim AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN test "$(node --version)" = "v26.12.1" \
	&& ! command -v curl \
	&& ! dpkg-query -W libbrotli1 >/dev/null 2>&1
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./package.json
COPY --from=build /app/dist ./dist
EXPOSE 3066
CMD ["node", "dist/index.js"]
