all: named # mr # consoled procd
	@echo "build done"

%:
	go build -o bin/$@ cmd/$@/main.go

run:
