TAILWIND_VERSION := v4.1.12
TAILWIND := bin/tailwindcss
TABLER_VERSION := 3.34.1
# Card chrome first, then the header icon set: Jira and GitHub issue types, and
# square-rotated, the fallback for a value no variant matches.
TABLER_ICONS := eye eye-off dots \
	square-rotated \
	bug checkbox bookmark bolt subtask sparkles trending-up target flask bulb \
	alert-triangle shield-lock help-circle book tool recycle gauge test-pipe rocket \
	circle-dot circle-check git-pull-request git-pull-request-draft \
	git-pull-request-closed git-merge messages

.PHONY: tools vendor generate build test fmt run

tools:
	@mkdir -p bin
	@os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case $$os in darwin) os=macos ;; esac; \
	case $$arch in x86_64) arch=x64 ;; esac; \
	curl --fail --location --output $(TAILWIND) \
		https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$$os-$$arch
	@chmod +x $(TAILWIND)

vendor:
	@mkdir -p internal/icons/svg
	@curl --fail --location --output internal/ui/static/htmx.min.js \
		https://unpkg.com/htmx.org@2.0.6/dist/htmx.min.js
	@for icon in $(TABLER_ICONS); do \
		curl --fail --silent --show-error --location \
			--output internal/icons/svg/$$icon.svg \
			https://unpkg.com/@tabler/icons@$(TABLER_VERSION)/icons/outline/$$icon.svg || exit 1; \
	done
	@curl --fail --silent --show-error --location \
		--output internal/icons/svg/LICENSE \
		https://unpkg.com/@tabler/icons@$(TABLER_VERSION)/LICENSE

generate:
	go tool templ generate
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/static/styles.css --minify

build:
	go build -o bin/multieditor .

test:
	go test ./...

fmt:
	gofmt -w .
	go tool templ fmt internal/ui

run:
	go run . --input examples/jira.yaml --output /tmp/multieditor-out.yaml --open
