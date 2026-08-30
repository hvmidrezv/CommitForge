# Go package docs (local guide)

Use this quick guide to inspect CommitForge docs from your terminal or browser.

## Terminal docs (`go doc`)

```sh
# Show docs for all packages in this module
go doc ./...

# Show docs for one package
go doc commitforge/internal/tui

# Show docs for one exported symbol
go doc commitforge/internal/tui.Model
```

## Browser docs (`godoc`)

```sh
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
# then open http://localhost:6060/pkg/commitforge/
```
