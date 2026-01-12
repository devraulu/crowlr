.PHONY: dev build run clean web build-web

dev:
	@$(MAKE) -s build
	@./tmp/crawler

build:
	go build -o ./tmp/crawler ./cmd/crawler/main.go

build-web:
	go build -o ./tmp/web ./cmd/web/main.go

run:
	./tmp/crawler

web:
	@$(MAKE) -s build-web
	@./tmp/web

clean:
	rm -rf ./tmp
