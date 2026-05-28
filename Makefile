SHELL := /bin/bash
TARGETS := chibi docs/chibi.1
VERSION := 0.1.2
SEMVER := $(shell echo $(VERSION) | sed 's/^v//')

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -o $@ $<

.PHONY: test
test:
	go test -v ./...

docs/chibi.1: docs/chibi.md Makefile
	@if command -v md2man-roff >/dev/null 2>&1; then \
		sed -e 's/{{DATE}}/'"$$(date +%F)"'/' -e 's/{{VERSION}}/$(SEMVER)/' docs/chibi.md | md2man-roff > docs/chibi.1; \
	else \
		echo "md2man-roff not installed, skipping man page generation"; \
	fi

# nfpm-based packaging.
.PHONY: deb
deb: $(TARGETS) docs/chibi.1
	SEMVER=$(SEMVER) GOARCH=amd64 nfpm package -p deb -f nfpm.yaml

.PHONY: rpm
rpm: $(TARGETS) docs/chibi.1
	SEMVER=$(SEMVER) GOARCH=amd64 nfpm package -p rpm -f nfpm.yaml

.PHONY: clean
clean:
	rm -f $(TARGETS)
	rm -f chibi_*.deb chibi-*.rpm

