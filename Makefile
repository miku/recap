SHELL := /bin/bash
TARGETS := recap
VERSION := 0.1.0
SEMVER := $(shell echo $(VERSION) | sed 's/^v//')

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -o $@ $<

.PHONY: test
test:
	go test -v ./...

docs/recap.1: docs/recap.md
	md2man-roff docs/recap.md > docs/recap.1

# nfpm-based packaging.
.PHONY: deb
deb: $(TARGETS) docs/recap.1
	SEMVER=$(SEMVER) GOARCH=amd64 nfpm package -p deb -f nfpm.yaml

.PHONY: rpm
rpm: $(TARGETS) docs/recap.1
	SEMVER=$(SEMVER) GOARCH=amd64 nfpm package -p rpm -f nfpm.yaml

.PHONY: clean
clean:
	rm -f $(TARGETS)
	rm -f $(TARGETS)_*.deb $(TARGETS)-*.rpm
	rm -f docs/recap.1

