.PHONY: build clean crawl install backfill web api

build:
	cd crawler && go build -o ./tmp/crawler ./cmd/crawler/

crawl:
	@$(MAKE) -s build
	cd crawler && ./tmp/crawler crawl

api:
	cd api && pnpm i && pnpm start

web: 
	cd web && pnpm dev

clean:
	cd crawler && rm -rf ./tmp
	cd api && pnpm i

install:
	cd web && pnpm i

backfill:
	@$(MAKE) -s build
	cd crawler && ./tmp/crawler backfill
