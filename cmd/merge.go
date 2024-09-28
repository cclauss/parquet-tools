package cmd

import (
	"fmt"
	"reflect"

	"github.com/xitongsys/parquet-go/reader"

	"github.com/hangxie/parquet-tools/internal"
)

// MergeCmd is a kong command for merge
type MergeCmd struct {
	internal.ReadOption
	internal.WriteOption
	ReadPageSize int      `help:"Page size to read from Parquet." default:"1000"`
	Sources      []string `help:"Files to be merged."`
	URI          string   `arg:"" predictor:"file" help:"URI of Parquet file."`
}

// Run does actual merge job
func (c MergeCmd) Run() error {
	if c.ReadPageSize < 1 {
		return fmt.Errorf("invalid read page size %d, needs to be at least 1", c.ReadPageSize)
	}
	if len(c.Sources) <= 1 {
		return fmt.Errorf("needs at least 2 sources files")
	}

	fileReaders, err := c.openSources()
	if err != nil {
		return err
	}
	defer func() {
		for _, fileReader := range fileReaders {
			_ = fileReader.PFile.Close()
		}
	}()
	schema := internal.NewSchemaTree(fileReaders[0]).JSONSchema()

	fileWriter, err := internal.NewGenericWriter(c.URI, c.WriteOption, schema)
	if err != nil {
		return fmt.Errorf("failed to write to [%s]: %v", c.URI, err)
	}
	defer func() {
		_ = fileWriter.WriteStop()
		_ = fileWriter.PFile.Close()
	}()

	for i := range fileReaders {
		for {
			rows, err := fileReaders[i].ReadByNumber(c.ReadPageSize)
			if err != nil {
				return fmt.Errorf("failed to read from [%s]: %v", c.Sources[i], err)
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				if err := fileWriter.Write(row); err != nil {
					return fmt.Errorf("failed to write data from [%s] to [%s]: %v", c.Sources[i], c.URI, err)
				}
			}
		}
	}
	if err := fileWriter.WriteStop(); err != nil {
		return fmt.Errorf("failed to end write [%s]: %v", c.URI, err)
	}
	if err := fileWriter.PFile.Close(); err != nil {
		return fmt.Errorf("failed to close [%s]: %v", c.URI, err)
	}

	return nil
}

func (c MergeCmd) openSources() ([]*reader.ParquetReader, error) {
	var schema *internal.SchemaNode
	var err error
	fileReaders := make([]*reader.ParquetReader, len(c.Sources))
	for i, source := range c.Sources {
		fileReaders[i], err = internal.NewParquetFileReader(source, c.ReadOption)
		if err != nil {
			return nil, fmt.Errorf("failed to read from [%s]: %v", source, err)
		}

		currSchema := internal.NewSchemaTree(fileReaders[i])
		if schema == nil {
			schema = currSchema
		} else if !reflect.DeepEqual(schema, currSchema) {
			return nil, fmt.Errorf("[%s] does not have same schema as previous files", source)
		}
	}

	return fileReaders, nil
}
