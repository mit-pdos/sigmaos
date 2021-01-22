all: bin/named bin/proxyd bin/schedd bin/submit bin/mr-m-wc bin/mr-r-wc bin/fsreader
	@echo "build done"

.PHONY: test
test:
	(cd memfs; go test -race)

%:
	go build -race -o bin/$(notdir $@) cmd/$(notdir $@)/main.go

.PHONY: clean
clean:
	rm bin/*
