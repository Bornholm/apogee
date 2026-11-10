# Apogée: the main development commands. `make` lists them.
#
# Variables can be overridden on the command line, for instance:
#   make server NAME="Ciel du bureau" SPEED=60
#   make balance SKIES=8 DAYS=6
#   make run ARGS="-zoom 2 -culture"

GO       ?= go
BIN      ?= bin
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

# Options of the targets below.
ARGS     ?=
NAME     ?= Apogée
DATA     ?= sky.json
SPEED    ?= 1
FAST     ?= 60
RIVALS   ?= 5
SKIES    ?= 16
DAYS     ?= 8
RAIDERS  ?= 0
SHOT     ?= shot.png
DOKKU_HOST ?= dev.lookingfora.name
DOKKU_APP  ?= apogee
OUT      ?= apogee-sound.wav

.DEFAULT_GOAL := help

.PHONY: help
help: ## List the targets
	@awk 'BEGIN {FS = ":.*## "} /^## / {printf "\n%s\n", substr($$0, 4)} /^[a-z0-9-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Play

.PHONY: run
run: ## Open the game on its home screen (ARGS adds options)
	$(GO) run ./cmd/apogee $(ARGS)

.PHONY: solo
solo: ## Continue the solo game, skipping the home screen
	$(GO) run ./cmd/apogee -solo $(ARGS)

.PHONY: fresh
fresh: ## Start a new solo game, skipping the home screen
	$(GO) run ./cmd/apogee -fresh $(ARGS)

.PHONY: fast
fast: ## A new solo game at ×FAST (60), to watch an island grow
	$(GO) run ./cmd/apogee -fresh -tutorial=false -speed $(FAST) $(ARGS)

.PHONY: server
server: ## Run a dedicated sky on the network (NAME, DATA, SPEED, RIVALS)
	$(GO) run ./cmd/apogee-server -name "$(NAME)" -data $(DATA) -speed $(SPEED) -rivals $(RIVALS) $(ARGS)

.PHONY: join
join: ## Join the first sky found on the network
	$(GO) run ./cmd/apogee -discover $(ARGS)

.PHONY: probe
probe: ## Print a few states of a sky found on the network
	$(GO) run ./cmd/apogee-probe $(ARGS)

## Check

.PHONY: test
test: ## Run the tests
	$(GO) test ./...

.PHONY: race
race: ## Run the tests with the race detector
	$(GO) test -race ./...

.PHONY: vet
vet: ## Vet the code and check its formatting
	$(GO) vet ./...
	@out=$$(gofmt -l cmd internal); if [ -n "$$out" ]; then echo "not formatted:"; echo "$$out"; exit 1; fi

.PHONY: fmt
fmt: ## Format the code
	gofmt -w cmd internal

.PHONY: check
check: vet test ## Vet, then test: run before committing

## Tune and preview

.PHONY: sim
sim: ## Simulate one island without rendering (ARGS: -hours, -pie)
	$(GO) run ./cmd/apogee-sim $(ARGS)

.PHONY: balance
balance: ## Run the balancing bench (SKIES, DAYS, RAIDERS)
	$(GO) run ./cmd/apogee-balance -skies $(SKIES) -days $(DAYS) -raiders $(RAIDERS) $(ARGS)

.PHONY: shot
shot: ## Save a screenshot of the solo game to SHOT (ARGS: -zoom 2 …)
	$(GO) run ./cmd/apogee -shot $(SHOT) $(ARGS)

.PHONY: art
art: ## Render the art preview to preview.png (ARGS: -view sky|icon)
	$(GO) run ./cmd/artpreview -o preview.png $(ARGS)

.PHONY: sound
sound: ## Render every effect and the music to OUT, a WAV file (ARGS: -ambience 8 for the ambiences)
	$(GO) run ./cmd/soundpreview -out $(OUT) $(ARGS)

## Build and release

.PHONY: build
build: ## Build the client and the server into BIN
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/ ./cmd/apogee ./cmd/apogee-server

.PHONY: snapshot
snapshot: ## Build a local release with GoReleaser, packages included, into dist
	goreleaser release --snapshot --clean

.PHONY: release-check
release-check: ## Validate the GoReleaser configuration
	goreleaser check

## Deploy

.PHONY: dokku-setup
dokku-setup: ## Create the Dokku app once: no proxy, TCP 47110 published, sky in storage
	ssh root@$(DOKKU_HOST) '\
		set -e; \
		dokku apps:exists $(DOKKU_APP) 2>/dev/null || dokku apps:create $(DOKKU_APP); \
		dokku builder:set $(DOKKU_APP) selected dockerfile; \
		dokku proxy:disable $(DOKKU_APP); \
		dokku checks:disable $(DOKKU_APP); \
		dokku storage:ensure-directory $(DOKKU_APP); \
		dokku storage:report $(DOKKU_APP) | grep -q /data || dokku storage:mount $(DOKKU_APP) /var/lib/dokku/data/storage/$(DOKKU_APP):/data; \
		dokku docker-options:report $(DOKKU_APP) | grep -q "47110:47110" || dokku docker-options:add $(DOKKU_APP) deploy "-p 47110:47110"; \
		dokku docker-options:report $(DOKKU_APP) | grep -q stop-timeout || dokku docker-options:add $(DOKKU_APP) deploy "--stop-timeout 30"; \
		dokku docker-options:report $(DOKKU_APP) | grep -q "build-arg GIT_REV" || dokku docker-options:add $(DOKKU_APP) build "--build-arg GIT_REV"'

.PHONY: dokku-deploy
dokku-deploy: ## Deploy HEAD to Dokku (the name: ssh dokku@DOKKU_HOST config:set APP APOGEE_NAME=…)
	$(if $(shell git config remote.dokku.url),, git remote add dokku dokku@$(DOKKU_HOST):$(DOKKU_APP))
	git push -f dokku $$(git rev-parse HEAD):refs/heads/master

.PHONY: clean
clean: ## Remove what the targets produced
	rm -rf $(BIN) dist preview.png $(SHOT) $(OUT)
