BIN     ?= lachesis-ui
PREFIX  ?= $(HOME)/.lachesis/bin
VERSION ?= 0.1.0
LDFLAGS ?= -X github.com/UnboundCompute/lachesis-ui/internal/mcp.Version=$(VERSION)

.PHONY: build install run tidy fmt vet check clean stack

## build the binary into the working directory
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

## build and drop the binary into ~/.lachesis/bin
install: build
	mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/$(BIN)

## run against the newest graph (pass ARGS="--graph ..." to override)
run: build
	./$(BIN) $(ARGS)

## pull + build the whole stack (engine, catalog, UI)
stack:
	./scripts/install.sh

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

## run the complete local gate used by CI
check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-clean:"; \
		echo "$$unformatted"; \
		echo "Run 'make fmt' and commit the result."; \
		exit 1; \
	fi
	go vet ./...
	go build ./...
	go test ./...

clean:
	rm -f $(BIN)
