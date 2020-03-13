VERSION ?= $(shell git describe --tags --abbrev=0 | sed 's/v//')
DEST ?= ./bin
SUFFIX?=""
TARGET_OS=linux darwin
TARGET_ARCH=amd64

export CGO_ENABLED=0

build:
	@echo "==> Build binaries..."
	go build -v -ldflags "-s -w -X main.version=${VERSION}" -o ${DEST}/sbus${SUFFIX} main.go

install: build
	cp -f ${DEST}/sbus /usr/local/bin/sbus
	chmod +x /usr/local/bin/sbus

dist:
	for GOOS in ${TARGET_OS}; do \
		for GOARCH in ${TARGET_ARCH}; do \
			GOOS=$$GOOS GOARCH=$$GOARCH SUFFIX=-v${VERSION}-$$GOOS-$$GOARCH make build; \
		done \
	done \

bump-tag:
	TAG=$$(echo "v${VERSION}" | awk -F. '{$$NF = $$NF + 1;} 1' | sed 's/ /./g'); \
	git tag $$TAG; \
	git push && git push --tags

release: dist
	@echo "==> Create github release and upload files..."

	-github-release release \
		--user servehub \
		--repo sbus-cli \
		--tag v${VERSION}

	for GOOS in ${TARGET_OS}; do \
		for GOARCH in ${TARGET_ARCH}; do \
			github-release upload \
				--user servehub \
				--repo sbus-cli \
				--tag v${VERSION} \
				--name sbus-v${VERSION}-$$GOOS-$$GOARCH \
				--file ${DEST}/sbus-v${VERSION}-$$GOOS-$$GOARCH \
				--replace; \
		done \
	done \
