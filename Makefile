all: named consoled procd sh # ls
	@echo "build done"

%:
	go build -o bin/$@ cmd/$@/main.go

run:
	
