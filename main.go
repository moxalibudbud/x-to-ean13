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
	prefix := flag.String("prefix", DefaultCompanyPrefix, "company prefix used to generate the EAN-13")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ean13 [-header] [-output <file>] <csv-file>")
		os.Exit(1)
	}

	inputPath := flag.Arg(0)
	outputPath := *output
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		outputPath = strings.TrimSuffix(inputPath, ext) + ".ean13" + ext
	}

	count, err := run(inputPath, outputPath, *prefix, *hasHeader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("ean13 file generated. %d row(s) to %s\n", count, outputPath)
}

// run reads inputPath, generating an EAN-13 for the first column of each row,
// and writes a two-column (word, ean13) CSV to outputPath. When hasHeader is
// true, the source's first-column header name is reused and "ean13" is
// appended to it.
func run(inputPath, outputPath, companyPrefix string, hasHeader bool) (int, error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	r := csv.NewReader(in)
	r.FieldsPerRecord = -1

	w := csv.NewWriter(out)
	defer w.Flush()

	if hasHeader {
		header, err := r.Read()
		if err != nil && err != io.EOF {
			return 0, fmt.Errorf("reading header: %w", err)
		}

		wordColumn := "word"
		if len(header) > 0 {
			wordColumn = header[0]
		}
		if err := w.Write([]string{wordColumn, "ean13"}); err != nil {
			return 0, fmt.Errorf("writing header: %w", err)
		}
	}

	count := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("reading record: %w", err)
		}
		if len(record) == 0 || record[0] == "" {
			continue
		}

		word := record[0]
		uuid := GenerateUUID(word)
		ean13, err := UUIDToEAN13(uuid, companyPrefix)
		if err != nil {
			return count, fmt.Errorf("generating EAN-13 for %q: %w", word, err)
		}

		if err := w.Write([]string{word, ean13}); err != nil {
			return count, fmt.Errorf("writing record: %w", err)
		}
		count++
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return count, err
	}

	return count, nil
}
