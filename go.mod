module github.com/Project-Arrakis/tabr-tau

go 1.26.11

require (
	// design is the nested module in ./design (see the replace line below), the shared look of the programs: the editor serves its files from /design/. The version is a placeholder, as for kit.
	github.com/Project-Arrakis/tabr-tau/design v0.0.0-00010101000000-000000000000
	// kit is the nested module in ./kit (see the replace line below). The version is a placeholder: the replace makes the code in the tree
	// the one that is built and tested. Nothing depends on this root module, so the replace never reaches another program.
	github.com/Project-Arrakis/tabr-tau/kit v0.0.0-00010101000000-000000000000
	github.com/jchv/go-webview2 v0.0.0-20260205173254-56598839c808
	modernc.org/sqlite v1.62.0
)

require (
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.49.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

replace github.com/Project-Arrakis/tabr-tau/design => ./design

replace github.com/Project-Arrakis/tabr-tau/kit => ./kit
