.PHONY: examples
examples:
	go run -gcflags=all="-N -l" ./cmd/kimerize examples
