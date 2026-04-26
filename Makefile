SHELL := /bin/bash
TARGETS := recap
VERSION := 0.1.0

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -o $@ $<

.PHONY: test
test:
	go test -v ./...

.PHONY: clean
clean:
	rm -f $(TARGETS)

