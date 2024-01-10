module github.com/fhs/acme-lsp

go 1.12

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/fhs/9fans-go v0.0.0-fhs.20200606
	github.com/fhs/go-lsp-internal v0.0.0-20230619000151-7826e273203a
	github.com/google/go-cmp v0.3.0
	github.com/sourcegraph/jsonrpc2 v0.2.0
)

// TODO: merge github.com/fhs/go-lsp-internal PR, update dependency and delete next line
replace github.com/fhs/go-lsp-internal => github.com/cloudspinner/go-lsp-internal v0.0.0-20240109201957-d9537c30fb78
