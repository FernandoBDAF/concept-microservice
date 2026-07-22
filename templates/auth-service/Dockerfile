# Stage 1: Builder
FROM node:22-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY tsconfig*.json ./
COPY src ./src
COPY migrations ./migrations

RUN npm run build
RUN npm prune --production

# Stage 2: Runner
FROM node:22-alpine AS runner

WORKDIR /app

RUN addgroup --system --gid 10001 nodejs && \
    adduser --system --uid 10001 nodejs

COPY --from=builder --chown=nodejs:nodejs /app/dist ./dist
COPY --from=builder --chown=nodejs:nodejs /app/node_modules ./node_modules
COPY --from=builder --chown=nodejs:nodejs /app/package.json ./
COPY --from=builder --chown=nodejs:nodejs /app/migrations ./migrations

USER 10001:10001

EXPOSE 3000

ENV NODE_ENV=production
ENV PORT=3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider "http://localhost:${PORT:-3000}/live" || exit 1

CMD ["node", "dist/server.js"]
