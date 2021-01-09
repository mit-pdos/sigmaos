all: named consoled mr # procd
	@echo "build done"

%:
	go build -o bin/$@ cmd/$@/main.go

run:


