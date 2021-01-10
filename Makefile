all: named consoled mr cntrlr # procd
	@echo "build done"

%:
	go build -o bin/$@ cmd/$@/main.go

run:


