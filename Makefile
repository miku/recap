SHELL := /bin/bash
TARGETS := recap docs/recap.1
VERSION := 0.1.1
SEMVER := $(shell echo $(VERSION) | sed 's/^v//')

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -o $@ $<

.PHONY: test
test:
	go test -v ./...

docs/recap.1: docs/recap.md
	@if command -v md2man-roff >/dev/null 2>&1; then \
		md2man-roff docs/recap.md > docs/recap.1; \
	else \
		echo "md2man-roff not installed, skipping man page generation"; \
	fi

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

