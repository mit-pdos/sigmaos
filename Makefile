all: named consoled mr cntrlr # procd
	@echo "build done"

test:
	(cd memfs; go test -race)

%:
	go build -race -o bin/$@ cmd/$@/main.go

run:


