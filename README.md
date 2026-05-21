# archived goproject

Move the archived project from the [goproject](https://github.com/futugyou/goproject)
repository to [this GitHub repository](https://github.com/futugyou/goproject-archived)

```sh
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
```
