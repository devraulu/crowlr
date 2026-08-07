
FROM node:24-alpine as base

WORKDIR /web

# ENV PNPM_HOME="/pnpm"
# ENV PATH="$PNPM_HOME/bin:$PATH"
RUN corepack enable pnpm && corepack install -g pnpm@latest-11

FROM base as prod

COPY pnpm-lock.yaml /.
RUN pnpm fetch --prod

COPY ./ /web
RUN pnpm build

FROM base
COPY --from=prod /node_modules /node_modules
COPY --from=prod /dist /dist
EXPOSE 3000
CMD ["pnpm", "start"]
