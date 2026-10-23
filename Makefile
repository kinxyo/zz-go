NAME := zz
MODULE := $(shell go list -m)

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

# direct imports of each package (own packages shown by name)
deps:
	@go list -f '{{.Name}}: {{join .Imports " "}}' ./... | sed 's#$(MODULE)/internal/##g'

.PHONY: run debug prod install deps
