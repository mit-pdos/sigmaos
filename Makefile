all: named consoled procd # sh
	@echo "build done"

%:
	go build -o bin/$@ cmd/$@/main.go

run:
