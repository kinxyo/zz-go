NAME := zz
MODULE := $(shell go list -m)

# where zz keeps its config (os.UserConfigDir)
ifeq ($(shell uname),Darwin)
CONFIG := $(HOME)/Library/Application Support/zz/setting.json
else
CONFIG := $(or $(XDG_CONFIG_HOME),$(HOME)/.config)/zz/setting.json
endif

run:
	go build -o $(NAME) ./cmd/zz
	@$(MAKE) install

debug:
	go run ./cmd/zz 2>&1 | less

prod:
	go build -trimpath -ldflags="-s -w" -o $(NAME) ./cmd/zz
	@$(MAKE) install

install:
	mkdir -p ~/bin
	rm -f ~/bin/$(NAME)
	mv ./$(NAME) ~/bin/$(NAME)

test:
	go vet ./...
	go test -race -count=1 ./...

# delete the config, project entries included; the next request writes a fresh one with the defaults
reset:
	rm -f "$(CONFIG)"

# direct imports of each package (own packages shown by name)
deps:
	@go list -f '{{.Name}}: {{join .Imports " "}}' ./... | sed 's#$(MODULE)/internal/##g'

.PHONY: run debug prod install test reset deps
