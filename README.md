# x-to-ean13

Generates a deterministic EAN-13 barcode for each word in a CSV file. The same
word always maps to the same EAN-13 (via a name-based UUIDv5, then converted
to EAN-13).

## Requirements

- Go 1.26+ (see `go.mod`)
- [`dlv`](https://github.com/go-delve/delve) if you want to debug (`go install github.com/go-delve/delve/cmd/dlv@latest`)

## Build

Compiles a binary named `x-to-ean13` in the current directory:

```sh
go build -o x-to-ean13 .
```

## Run without building (compile + run in one step)

```sh
go run . [-header] [-prefix <company-prefix>] [-output <file>] <csv-file>
```

## Usage

| Flag       | Default                  | Description                                      |
|------------|--------------------------|---------------------------------------------------|
| `-header`  | `false`                  | Set if the input CSV has a header row to skip     |
| `-prefix`  | `200`                    | Company prefix used when generating the EAN-13    |
| `-output`  | `<input>.ean13.csv`      | Output CSV file path                              |

The first column of each input row is treated as the word. The output CSV has
two columns: the word and its generated `ean13`. If `-header` is set, the
output header reuses the source's first-column name and appends `ean13`.

Example:

```sh
./x-to-ean13 -header -prefix 200 words.csv
# wrote N row(s) to words.ean13.csv
```

## Test

```sh
go test ./...
```

## Debug

Using [Delve](https://github.com/go-delve/delve) from the command line, passing
program flags after `--`:

```sh
dlv debug . -- -header words.csv
```

Common Delve commands once stopped at a breakpoint: `b main.run` (set a
breakpoint), `c` (continue), `n` (step over), `p <var>` (print a variable),
`bt` (stack trace).

### VS Code

Add a launch configuration (`.vscode/launch.json`) such as:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug x-to-ean13",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "args": ["-header", "words.csv"]
    }
  ]
}
```

Then set breakpoints in the editor and use "Run and Debug".
