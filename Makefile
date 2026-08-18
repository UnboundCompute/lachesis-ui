BIN     ?= lachesis-ui
PREFIX  ?= $(HOME)/.lachesis/bin

.PHONY: build install run tidy fmt vet clean stack

## build the binary into the working directory
build:
	go build -o $(BIN) .

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

clean:
	rm -f $(BIN)
