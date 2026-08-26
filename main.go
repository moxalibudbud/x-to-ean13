package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	hasHeader := flag.Bool("header", false, "set if the input CSV file has a header row to skip")
	output := flag.String("output", "", "output CSV file path (default: <input>.ean13.csv)")
	ledgerPath := flag.String("ledger", "", "ledger file recording key->EAN-13 allocations, so codes stay stable across runs (default: <input>.ledger.csv)")
	prefix := flag.String("prefix", DefaultCompanyPrefix, "company prefix used to generate the EAN-13")
	flag.Parse()

	if err := ValidateCompanyPrefix(*prefix); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if class := ClassifyPrefix(*prefix); class != PrefixRestricted {
		fmt.Fprintf(os.Stderr,
			"warning: company prefix %q is a %s; codes generated from it are not guaranteed to be globally unique unless you hold a GS1 license for it\n",
			*prefix, class)
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ean13 [-header] [-output <file>] [-ledger <file>] [-prefix <prefix>] <csv-file>")
		os.Exit(1)
	}

	inputPath := flag.Arg(0)
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)

	outputPath := *output
	if outputPath == "" {
		outputPath = base + ".ean13" + ext
	}

	ledgerFile := *ledgerPath
	if ledgerFile == "" {
		ledgerFile = base + ".ledger" + ext
	}

	result, err := run(inputPath, outputPath, ledgerFile, *prefix, *hasHeader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("ean13 file generated. %d row(s) to %s (%d newly allocated, %d reused from %s)\n",
		result.total, outputPath, result.allocated, result.total-result.allocated, ledgerFile)
}

type runResult struct {
	total     int
	allocated int
}

// run reads inputPath, allocates an EAN-13 for the first column of each row
// via the ledger at ledgerPath, and writes a two-column (word, ean13) CSV to
// outputPath. When hasHeader is true, the source's first-column header name is
// reused and "ean13" is appended to it.
//
// All allocations are made and the ledger is saved before the output CSV is
// written, so a failure partway through never leaves the ledger disagreeing
// with a partially written output file.
func run(inputPath, outputPath, ledgerPath, companyPrefix string, hasHeader bool) (runResult, error) {
	ledger, err := LoadLedger(ledgerPath, companyPrefix)
	if err != nil {
		return runResult{}, fmt.Errorf("loading ledger: %w", err)
	}

	in, err := os.Open(inputPath)
	if err != nil {
		return runResult{}, err
	}
	defer in.Close()

	r := csv.NewReader(in)
	r.FieldsPerRecord = -1

	wordColumn := "word"
	if hasHeader {
		header, err := r.Read()
		if err != nil && err != io.EOF {
			return runResult{}, fmt.Errorf("reading header: %w", err)
		}
		if len(header) > 0 {
			wordColumn = header[0]
		}
	}

	type row struct {
		word  string
		ean13 string
	}

	var rows []row
	result := runResult{}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return runResult{}, fmt.Errorf("reading record: %w", err)
		}
		if len(record) == 0 || record[0] == "" {
			continue
		}

		word := record[0]
		ean13, isNew, err := ledger.Allocate(word)
		if err != nil {
			return runResult{}, fmt.Errorf("allocating EAN-13 for %q: %w", word, err)
		}
		if isNew {
			result.allocated++
		}

		rows = append(rows, row{word: word, ean13: ean13})
		result.total++
	}

	if err := ledger.Save(); err != nil {
		return runResult{}, fmt.Errorf("saving ledger: %w", err)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return runResult{}, err
	}
	defer out.Close()

	w := csv.NewWriter(out)

	if hasHeader {
		if err := w.Write([]string{wordColumn, "ean13"}); err != nil {
			return runResult{}, fmt.Errorf("writing header: %w", err)
		}
	}
	for _, rw := range rows {
		if err := w.Write([]string{rw.word, rw.ean13}); err != nil {
			return runResult{}, fmt.Errorf("writing record: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return runResult{}, err
	}

	return result, nil
}
