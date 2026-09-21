FROM node:26.7.0 AS deps
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* ./
RUN npm ci --legacy-peer-deps --no-audit --no-fund

FROM deps AS build
WORKDIR /app
COPY tsconfig.json ./
COPY src ./src
RUN npx tsc -p .

FROM node:26.7.0 AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./package.json
COPY --from=build /app/dist ./dist
EXPOSE 3066
CMD ["node", "dist/index.js"]
