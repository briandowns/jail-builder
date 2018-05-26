GO ?= go
GLIDE ?= glide

.PHONY:
deps:
ifeq (,$(wildcard glide.yaml))
	$(GLIDE) init
else
	$(GLIDE) update
endif

.PHONY:
test:
	$(GO) test -v . -cover

.PHONY:
clean:
	$(GO) clean
