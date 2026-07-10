FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* ./
RUN npm ci --legacy-peer-deps || npm i --legacy-peer-deps

FROM deps AS build
WORKDIR /app
COPY tsconfig.json ./
COPY src ./src
RUN npx tsc -p .

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./package.json
COPY --from=build /app/dist ./dist
EXPOSE 3066
CMD ["node", "dist/index.js"]