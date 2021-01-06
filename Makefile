all: named # consoled procd 9pd sh ls
	@echo "build done"

%:
	go build -o bin/$@ cmd/$@/main.go

run:
