all: named consoled mr cntrlr # procd
	@echo "build done"

%:
	go build -race -o bin/$@ cmd/$@/main.go

run:


