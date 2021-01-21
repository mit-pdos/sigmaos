all: named proxyd schedd submit mr-m-wc mr-r-wc fsreader
	@echo "build done"

.PHONY: test
test:
	(cd memfs; go test -race)

%:
	go build -race -o bin/$@ cmd/$@/main.go

run:
