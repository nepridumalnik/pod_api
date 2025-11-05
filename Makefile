all: gen

gen:
	 go generate ./...

tidy:
	go mod tidy
