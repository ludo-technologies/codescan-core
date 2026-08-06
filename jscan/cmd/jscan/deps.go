package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/service"
	"github.com/spf13/cobra"
)

var (
	depsOutputFormat    string
	depsOutputPath      string
	depsConfigPath      string
	depsDotFormat       bool
	depsIncludeExternal bool
	depsIncludeTypes    bool
	depsNoCycles        bool
	depsMaxDepth        int
	depsMinCoupling     int
	depsNoLegend        bool
	depsRankDir         string
)

func depsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps [path...]",
		Short: "Analyze and visualize module dependencies",
		Long: `Analyze JavaScript/TypeScript module dependencies and generate visualizations.

Supports multiple output formats:
  - text: Human-readable text summary
  - json: JSON format for programmatic consumption
  - dot:  Graphviz DOT format for visualization

Examples:
  # Generate DOT and render with Graphviz
  jscan deps --dot src/ > deps.dot
  dot -Tpng deps.dot -o deps.png

  # Pipe directly to Graphviz
  jscan deps --dot src/ | dot -Tsvg -o deps.svg

  # Filter by coupling
  jscan deps --dot --min-coupling 5 src/

  # JSON for programmatic use
  jscan deps --format json src/

  # Save to file
  jscan deps --dot -o deps.dot src/`,
		RunE: runDeps,
	}

	cmd.Flags().StringVarP(&depsOutputFormat, "format", "f", "text",
		"Output format: text, json, dot")
	cmd.Flags().StringVarP(&depsOutputPath, "output", "o", "",
		"Output file path (default: stdout)")
	cmd.Flags().StringVarP(&depsConfigPath, "config", "c", "",
		"Path to config file")
	cmd.Flags().BoolVar(&depsDotFormat, "dot", false,
		"Shorthand for --format dot")
	cmd.Flags().BoolVar(&depsIncludeExternal, "include-external", false,
		"Include node_modules dependencies")
	cmd.Flags().BoolVar(&depsIncludeTypes, "include-types", true,
		"Include TypeScript type imports")
	cmd.Flags().BoolVar(&depsNoCycles, "no-cycles", false,
		"Disable cycle detection")
	cmd.Flags().IntVar(&depsMaxDepth, "max-depth", 0,
		"Limit dependency depth shown (0 = unlimited)")
	cmd.Flags().IntVar(&depsMinCoupling, "min-coupling", 0,
		"Only show nodes with coupling >= N")
	cmd.Flags().BoolVar(&depsNoLegend, "no-legend", false,
		"Disable legend in DOT output")
	cmd.Flags().StringVar(&depsRankDir, "rank-dir", "TB",
		"Layout direction for DOT: TB, LR, BT, RL")

	return cmd
}

func runDeps(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("no paths specified")
	}

	// Determine output format
	format := domain.OutputFormatText
	if depsDotFormat || depsOutputFormat == "dot" {
		format = domain.OutputFormatDOT
	} else if depsOutputFormat == "json" {
		format = domain.OutputFormatJSON
	} else if depsOutputFormat == "text" {
		format = domain.OutputFormatText
	}

	// Load configuration
	cfg, err := loadCommandConfig(depsConfigPath, args[0], os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Collect JavaScript/TypeScript files (using the patterns from config)
	var files []string
	for _, path := range args {
		pathFiles, err := collectJSFiles(path, cfg)
		if err != nil {
			return fmt.Errorf("failed to collect files from %s: %w", path, err)
		}
		files = append(files, pathFiles...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no JavaScript/TypeScript files found")
	}

	if format != domain.OutputFormatJSON && format != domain.OutputFormatDOT {
		fmt.Printf("Analyzing %d files...\n", len(files))
	}

	// Create dependency graph service
	svc := service.NewDependencyGraphService(depsIncludeExternal, depsIncludeTypes)

	// Build request
	req := domain.DependencyGraphRequest{
		Paths:              files,
		OutputFormat:       format,
		IncludeExternal:    domain.BoolPtr(depsIncludeExternal),
		IncludeTypeImports: domain.BoolPtr(depsIncludeTypes),
		DetectCycles:       domain.BoolPtr(!depsNoCycles),
	}

	// Analyze
	ctx := context.Background()
	startTime := time.Now()
	response, err := svc.Analyze(ctx, req)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	duration := time.Since(startTime)

	// Print warnings/errors if not JSON/DOT output
	if format == domain.OutputFormatText {
		for _, w := range response.Warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
		}
		for _, e := range response.Errors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", e)
		}
	}

	// Determine output writer
	var writer *os.File
	if depsOutputPath != "" {
		f, createErr := os.Create(depsOutputPath)
		if createErr != nil {
			return fmt.Errorf("failed to create output file: %w", createErr)
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("failed to close output file: %w", closeErr)
			}
		}()
		writer = f
	} else {
		writer = os.Stdout
	}

	// Format output
	formatter := service.NewOutputFormatter()
	switch format {
	case domain.OutputFormatDOT:
		dotConfig := service.DefaultDOTFormatterConfig()
		dotConfig.MaxDepth = depsMaxDepth
		dotConfig.MinCoupling = depsMinCoupling
		dotConfig.ShowLegend = !depsNoLegend
		dotConfig.ClusterCycles = !depsNoCycles
		dotConfig.RankDir = depsRankDir

		dotFormatter := service.NewDOTFormatter(dotConfig)
		if err := dotFormatter.WriteDependencyGraph(response, writer); err != nil {
			return fmt.Errorf("failed to write DOT output: %w", err)
		}

	case domain.OutputFormatJSON:
		if err := formatter.WriteDependencyGraph(response, format, writer); err != nil {
			return fmt.Errorf("failed to write JSON output: %w", err)
		}

	default:
		if err := formatter.WriteDependencyGraph(response, format, writer); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(writer, "\nAnalysis completed in %dms\n", duration.Milliseconds())
	}

	// Print output path if writing to file
	if depsOutputPath != "" && format != domain.OutputFormatJSON && format != domain.OutputFormatDOT {
		absPath, _ := filepath.Abs(depsOutputPath)
		fmt.Printf("Output saved to: %s\n", absPath)
	}

	return nil
}
