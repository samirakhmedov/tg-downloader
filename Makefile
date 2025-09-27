init:
	make clean

	make install

	make config-gen

	make db-gen
	
	go mod tidy

db-gen:
	go generate ./ent

tidy:
	go mod tidy

config-gen:
	rm -rf env

	pkl eval config/Config.pkl

	pkl-gen-go config/Config.pkl

install:
	go mod download

build:
	go build -o build/tg-downloader .

run:
	rm -rf build

	make build

	./build/tg-downloader

clean:
	rm -rf build
	rm -rf env
	rm -rf go.sum