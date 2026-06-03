TAILWIND ?= ./tailwindcss
CSS_IN := web/static/css/input.css
CSS_OUT := web/static/css/app.css
TW_CONFIG := tailwind.theralert.config.js

.PHONY: css css-watch build run dev tidy

css:
	$(TAILWIND) -c $(TW_CONFIG) -i $(CSS_IN) -o $(CSS_OUT) --minify

css-watch:
	$(TAILWIND) -c $(TW_CONFIG) -i $(CSS_IN) -o $(CSS_OUT) --watch

build: css
	go build -ldflags="-s -w" -o theralert ./cmd/server

run: css
	go run ./cmd/server

# Rebuild CSS then run (handy during development).
dev: css run

tidy:
	go mod tidy
