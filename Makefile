all: named consoled mrwc cntrlr proxyd # procd
	@echo "build done"

.PHONY: test
test:
	(cd memfs; go test -race)

%:
	go build -race -o bin/$@ cmd/$@/main.go

run:
