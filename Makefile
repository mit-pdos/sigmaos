all: named proxyd consoled mr-m-wc mr-r-wc ulambd fsreader
	@echo "build done"

.PHONY: test
test:
	(cd memfs; go test -race)

%:
	go build -race -o bin/$@ cmd/$@/main.go

run:
