package interp

import (
	"encoding/csv"
	"runtime"
	"strings"
)

func formatCSV(separator rune, fields []string) string {
	// TODO: extremely inefficient, just for a proof of concept
	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.Comma = separator
	writer.UseCRLF = runtime.GOOS == "windows"
	err := writer.Write(fields)
	if err != nil {
		return ""
	}
	writer.Flush()
	if writer.Error() != nil {
		return ""
	}
	return output.String()
}
