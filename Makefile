
SRCS := $(shell find . -name "*.go" 2>/dev/null)

all: corax

config.go:
	@cp config.def $@

corax: config.go ${SRCS}
	go build -o bin/corax .

clean: 
	rm -rf bin/ && rm config.go
# 	TODO : make dist command
#  TODO : make install command and uninstall

.PHONY: all clean