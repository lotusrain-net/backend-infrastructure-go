ARG IMAGE_REGISTRY=public.ecr.aws/docker
FROM ${IMAGE_REGISTRY}/library/node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM ${IMAGE_REGISTRY}/library/node:20-alpine AS builder
WORKDIR /app
ARG API_PROXY_TARGET
ENV API_PROXY_TARGET=$API_PROXY_TARGET
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM ${IMAGE_REGISTRY}/library/node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
ENV NEXT_TELEMETRY_DISABLED=1
ARG API_PROXY_TARGET
ENV API_PROXY_TARGET=$API_PROXY_TARGET
RUN addgroup -S -g 10001 nextjs && adduser -S -u 10001 -G nextjs nextjs
COPY --from=builder --chown=10001:10001 /app/.next/standalone ./
COPY --from=builder --chown=10001:10001 /app/.next/static ./.next/static
USER 10001:10001
EXPOSE 3000
CMD ["node", "server.js"]
