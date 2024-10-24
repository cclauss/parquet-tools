package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/hangxie/parquet-tools/internal"
)

// ImportCmd is a kong command for import
type ImportCmd struct {
	internal.WriteOption
	Source     string `required:"" short:"s" predictor:"file" help:"Source file name."`
	Format     string `help:"Source file formats (csv/json/jsonl)." short:"f" enum:"csv,json,jsonl" default:"csv"`
	Schema     string `required:"" short:"m" predictor:"file" help:"Schema file name."`
	SkipHeader bool   `help:"Skip first line of CSV files" default:"false"`
	URI        string `arg:"" predictor:"file" help:"URI of Parquet file."`
}

// Run does actual import job
func (c ImportCmd) Run() error {
	switch c.Format {
	case "csv":
		return c.importCSV()
	case "json":
		return c.importJSON()
	case "jsonl":
		return c.importJSONL()
	}
	return fmt.Errorf("[%s] is not a recognized source format", c.Format)
}

func (c ImportCmd) importCSV() error {
	schemaData, err := os.ReadFile(c.Schema)
	if err != nil {
		return fmt.Errorf("failed to load schema from %s: %w", c.Schema, err)
	}
	if matched, _ := regexp.Match(`(?i)type\s*=\s*int96`, schemaData); matched {
		return fmt.Errorf("import does not support INT96 type")
	}

	schema := []string{}
	for _, line := range strings.Split(string(schemaData), "\n") {
		line = strings.Trim(line, "\r\n\t ")
		if line == "" {
			continue
		}
		schema = append(schema, line)
	}

	csvFile, err := os.Open(c.Source)
	if err != nil {
		return fmt.Errorf("failed to open CSV file %s: %w", c.Source, err)
	}
	defer csvFile.Close()
	csvReader := csv.NewReader(csvFile)

	parquetWriter, err := internal.NewCSVWriter(c.URI, c.WriteOption, schema)
	if err != nil {
		return fmt.Errorf("failed to create CSV writer: %w", err)
	}

	if c.SkipHeader {
		_, _ = csvReader.Read()
	}
	for {
		fields, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		parquetFields := make([]*string, len(fields))
		for i := 0; i < len(fields); i++ {
			parquetFields[i] = &fields[i]
		}
		if err = parquetWriter.WriteString(parquetFields); err != nil {
			return fmt.Errorf("failed to write [%v] to parquet: %w", fields, err)
		}
	}
	if err := parquetWriter.WriteStop(); err != nil {
		return fmt.Errorf("failed to close Parquet writer %s: %w", c.URI, err)
	}
	if err := parquetWriter.PFile.Close(); err != nil {
		return fmt.Errorf("failed to close Parquet file %s: %w", c.URI, err)
	}

	return nil
}

func (c ImportCmd) importJSON() error {
	schemaData, err := os.ReadFile(c.Schema)
	if err != nil {
		return fmt.Errorf("failed to load schema from %s: %w", c.Schema, err)
	}
	if matched, _ := regexp.Match(`(?i)type\s*=\s*int96`, schemaData); matched {
		return fmt.Errorf("import does not support INT96 type")
	}

	jsonData, err := os.ReadFile(c.Source)
	if err != nil {
		return fmt.Errorf("failed to load source from %s: %w", c.Source, err)
	}

	var dummy map[string]interface{}
	if err := json.Unmarshal([]byte(schemaData), &dummy); err != nil {
		return fmt.Errorf("content of %s is not a valid schema JSON", c.Schema)
	}
	if err := json.Unmarshal(jsonData, &dummy); err != nil {
		return fmt.Errorf("invalid JSON string: %s", string(jsonData))
	}

	parquetWriter, err := internal.NewJSONWriter(c.URI, c.WriteOption, string(schemaData))
	if err != nil {
		return fmt.Errorf("failed to create JSON writer: %w", err)
	}

	if err := parquetWriter.Write(string(jsonData)); err != nil {
		return fmt.Errorf("failed to write to parquet file: %w", err)
	}

	if err := parquetWriter.WriteStop(); err != nil {
		return fmt.Errorf("failed to close Parquet writer %s: %w", c.URI, err)
	}
	if err := parquetWriter.PFile.Close(); err != nil {
		return fmt.Errorf("failed to close Parquet file %s: %w", c.URI, err)
	}

	return nil
}

func (c ImportCmd) importJSONL() error {
	schemaData, err := os.ReadFile(c.Schema)
	if err != nil {
		return fmt.Errorf("failed to load schema from %s: %w", c.Schema, err)
	}
	if matched, _ := regexp.Match(`(?i)type\s*=\s*int96`, schemaData); matched {
		return fmt.Errorf("import does not support INT96 type")
	}

	var dummy map[string]interface{}
	if err := json.Unmarshal([]byte(schemaData), &dummy); err != nil {
		return fmt.Errorf("content of %s is not a valid schema JSON", c.Schema)
	}

	jsonlFile, err := os.Open(c.Source)
	if err != nil {
		return fmt.Errorf("failed to open source file %s", c.Source)
	}
	defer jsonlFile.Close()
	scanner := bufio.NewScanner(jsonlFile)
	scanner.Split(bufio.ScanLines)

	parquetWriter, err := internal.NewJSONWriter(c.URI, c.WriteOption, string(schemaData))
	if err != nil {
		return fmt.Errorf("failed to create JSON writer: %w", err)
	}

	for scanner.Scan() {
		jsonData := scanner.Bytes()
		if err := json.Unmarshal(jsonData, &dummy); err != nil {
			return fmt.Errorf("invalid JSON string: %s", string(jsonData))
		}

		if err := parquetWriter.Write(string(jsonData)); err != nil {
			return fmt.Errorf("failed to write to parquet file: %w", err)
		}
	}
	if err := parquetWriter.WriteStop(); err != nil {
		return fmt.Errorf("failed to close Parquet writer %s: %w", c.URI, err)
	}
	if err := parquetWriter.PFile.Close(); err != nil {
		return fmt.Errorf("failed to close Parquet file %s: %w", c.URI, err)
	}

	return nil
}
