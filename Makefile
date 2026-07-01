.PHONY: build clean crawl py-install py-backfill py-web

build:
	go build -o ./tmp/crawler ./cmd/crawler/

crawl:
	@$(MAKE) -s build
	./tmp/crawler crawl

clean:
	rm -rf ./tmp

py-install:
	cd python && uv sync

py-backfill:
	cd python && uv run python -m crowlr_py.backfill.run

py-web:
	cd python && uv run uvicorn crowlr_py.web.app:app --host 0.0.0.0 --port 8080
