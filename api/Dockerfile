FROM node:26-alpine

WORKDIR /api

ENV NODE_ENV=production

RUN apk add --no-cache pnpm>=11.13.0 curl

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./

RUN pnpm install --frozen-lockfile

COPY . .

EXPOSE 8080

CMD ["pnpm", "start"]
