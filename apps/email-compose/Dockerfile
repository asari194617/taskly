# syntax=docker/dockerfile:1.6

FROM node:26.12.1 AS deps
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

FROM node:26.12.1 AS runner
WORKDIR /app
ENV NODE_ENV=production
# Install production deps only
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
	npm ci --omit=dev --no-audit --no-fund --ignore-scripts
COPY --from=build /app/dist ./dist
EXPOSE 50051 50052
CMD ["node", "dist/index.js"]
