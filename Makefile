.PHONY: clean build test cover install system-prompt 

build: system-prompt
	go build -ldflags="-s -w -X main.buildTime=`date -u +'%Y-%m-%dT%H:%M:%SZ'`" -o llm ./cmd/llm

test:
	go test ./... -coverprofile=coverage.out ./...

cover: test
	go tool cover -html=coverage.out

install:
	go install ./cmd/llm

system-prompt:
	processed_prompt=$$(cat system_prompt.md | tr -d '\n' | sed 's/\\n/\\\\n/g'); \
	grep -v '^system_prompt=' example.llmrc > example.llmrc.tmp; \
	echo "system_prompt=$${processed_prompt}" >> example.llmrc.tmp; \
	mv example.llmrc.tmp example.llmrc

clean:
	rm -f coverage.out
	rm -f generated_*
	rm -f llm
