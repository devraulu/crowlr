.PHONY: build clean crawl install backfill web api

build:
	go build -o ./tmp/crawler ./cmd/crawler/

crawl:
	@$(MAKE) -s build
	./tmp/crawler crawl

clean:
	rm -rf ./tmp

install:
	go mod tidy
