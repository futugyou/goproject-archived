module github.com/futugyou/semantic_kernel

go 1.26.2

require (
	github.com/beevik/etree v1.6.0
	github.com/futugyou/extensions_ai v0.0.1
	github.com/futugyou/kernel_memory v0.0.1
	github.com/futugyou/yomawari v0.0.1
)

replace github.com/futugyou/yomawari v0.0.1 => ../yomawari

replace github.com/futugyou/extensions_ai v0.0.1 => ../extensions_ai

replace github.com/futugyou/kernel_memory v0.0.1 => ../kernel_memory
