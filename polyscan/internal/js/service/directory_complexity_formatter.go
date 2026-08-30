package service

import (
	"fmt"
	"io"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// writeDirectoryComplexityText renders the directory rollups as plain text.
// Directories arrive ranked worst-first, so a reader who stops early has still
// seen the ones worth acting on.
func writeDirectoryComplexityText(writer io.Writer, directories domain.DirectoryComplexityMetricsList) {
	if len(directories) == 0 {
		return
	}

	fmt.Fprintf(writer, "Directory Complexity:\n")
	for _, directory := range directories {
		fmt.Fprintf(writer, "  %s\n", directory.DirectoryPath)
		fmt.Fprintf(writer, "    Functions: %d\n", directory.FunctionCount)
		fmt.Fprintf(writer, "    Complexity: avg %.2f, max %d, high-risk %d\n",
			directory.AverageComplexity, directory.MaxComplexity, directory.HighRiskFunctionCount)
		fmt.Fprintf(writer, "    Nesting: avg %.2f, max %d\n",
			directory.AverageNestingDepth, directory.MaxNestingDepth)
	}
	fmt.Fprintf(writer, "\n")
}
