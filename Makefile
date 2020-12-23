all:
	go build -o bin/named cmd/named/main.go
	go build -o bin/consoled cmd/consoled/main.go
	go build -o bin/procd cmd/procd/main.go
	# go build -o login cmd/login/main.go
	go build -o bin/gsh cmd/sh/main.go
