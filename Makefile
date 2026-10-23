MODULE := $(shell go list -m)

install:
	go build ./cmd/zz
	mkdir -p ~/bin
	rm -f ~/bin/zz
	mv ./zz ~/bin/

run:
	go run ./cmd/zz

# direct imports of each package (own packages shown by name)
deps:
	@go list -f '{{.Name}}: {{join .Imports " "}}' ./... | sed 's#$(MODULE)/internal/##g'
