all: named consoled mr cntrlr proxyd # procd
	@echo "build done"

test:
	(cd memfs; go test -race)

%:
	go build -race -o bin/$@ cmd/$@/main.go

run:
