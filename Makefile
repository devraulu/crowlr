.PHONY: dev build run clean web

dev:
	@$(MAKE) -s build
	@./tmp/crawler crawl

build:
	go build -o ./tmp/crawler ./cmd/crawler/

run:
	./tmp/crawler crawl

web:
	@$(MAKE) -s build
	@./tmp/crawler web

clean:
	rm -rf ./tmp
