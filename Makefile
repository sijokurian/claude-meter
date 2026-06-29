.PHONY: build clean

build:
	go build -o claude-meter .

clean:
	rm -f claude-meter claude-meter-*
