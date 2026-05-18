module github.com/futugyousuzu/graphify

go 1.26.2

require (
	github.com/dominikbraun/graph v0.23.0
	github.com/fsnotify/fsnotify v1.10.1
	github.com/futugyou/yomawari v0.0.1
	github.com/google/uuid v1.6.0
)

require golang.org/x/sys v0.13.0 // indirect

replace github.com/futugyou/yomawari v0.0.1 => ../yomawari
