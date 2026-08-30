package analyzer

import (
	"strings"
	"time"

	corecfg "github.com/ludo-technologies/polyscan/core/cfg"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// SeverityLevel represents the severity of a dead code finding
type SeverityLevel string

const (
	// SeverityLevelCritical indicates code that is definitely unreachable
	SeverityLevelCritical SeverityLevel = "critical"

	// SeverityLevelWarning indicates code that is likely unreachable
	SeverityLevelWarning SeverityLevel = "warning"

	// SeverityLevelInfo indicates potential optimization opportunities
	SeverityLevelInfo SeverityLevel = "info"
)

// DeadCodeReason represents the reason why code is considered dead
type DeadCodeReason string

const (
	// ReasonUnreachableAfterReturn indicates code after a return statement
	ReasonUnreachableAfterReturn DeadCodeReason = "unreachable_after_return"

	// ReasonUnreachableAfterBreak indicates code after a break statement
	ReasonUnreachableAfterBreak DeadCodeReason = "unreachable_after_break"

	// ReasonUnreachableAfterContinue indicates code after a continue statement
	ReasonUnreachableAfterContinue DeadCodeReason = "unreachable_after_continue"

	// ReasonUnreachableAfterThrow indicates code after a throw statement
	ReasonUnreachableAfterThrow DeadCodeReason = "unreachable_after_throw"

	// ReasonUnreachableBranch indicates an unreachable branch condition
	ReasonUnreachableBranch DeadCodeReason = "unreachable_branch"

	// ReasonUnreachableAfterInfiniteLoop indicates code after an infinite loop
	ReasonUnreachableAfterInfiniteLoop DeadCodeReason = "unreachable_after_infinite_loop"

	// ReasonUnusedImport indicates an imported name that is never referenced
	ReasonUnusedImport DeadCodeReason = "unused_import"

	// ReasonUnusedExport indicates an exported name that is never imported by other files
	ReasonUnusedExport DeadCodeReason = "unused_export"

	// ReasonOrphanFile indicates a file that is not reachable from any entry point via imports
	ReasonOrphanFile DeadCodeReason = "orphan_file"

	// ReasonUnusedExportedFunction indicates an exported function/class that is not imported by any other file
	ReasonUnusedExportedFunction DeadCodeReason = "unused_exported_function"
)

// DeadCodeFinding represents a single dead code detection result
type DeadCodeFinding struct {
	// Function information
	FunctionName string `json:"function_name"`
	FilePath     string `json:"file_path"`

	// Location information
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`

	// Dead code details
	BlockID     string         `json:"block_id"`
	Code        string         `json:"code"`
	Reason      DeadCodeReason `json:"reason"`
	Severity    SeverityLevel  `json:"severity"`
	Description string         `json:"description"`

	// Context information
	Context []string `json:"context,omitempty"`
}

// DeadCodeResult contains the results of dead code analysis for a single CFG
type DeadCodeResult struct {
	// Function information
	FunctionName string `json:"function_name"`
	FilePath     string `json:"file_path"`

	// Analysis results
	Findings       []*DeadCodeFinding `json:"findings"`
	TotalBlocks    int                `json:"total_blocks"`
	DeadBlocks     int                `json:"dead_blocks"`
	ReachableRatio float64            `json:"reachable_ratio"`

	// Performance metrics
	AnalysisTime time.Duration `json:"analysis_time"`
}

// DeadCodeDetector provides high-level dead code detection functionality
type DeadCodeDetector struct {
	cfg      *CFG
	filePath string // File path for context in findings
}

// NewDeadCodeDetector creates a new dead code detector for the given CFG
func NewDeadCodeDetector(cfg *CFG) *DeadCodeDetector {
	return &DeadCodeDetector{
		cfg:      cfg,
		filePath: "",
	}
}

// NewDeadCodeDetectorWithFilePath creates a new dead code detector with file path context
func NewDeadCodeDetectorWithFilePath(cfg *CFG, filePath string) *DeadCodeDetector {
	return &DeadCodeDetector{
		cfg:      cfg,
		filePath: filePath,
	}
}

// Detect performs dead code detection and returns structured findings
func (dcd *DeadCodeDetector) Detect() *DeadCodeResult {
	startTime := time.Now()

	result := &DeadCodeResult{
		FunctionName: dcd.getFunctionName(),
		FilePath:     dcd.getFilePath(),
		Findings:     make([]*DeadCodeFinding, 0),
		TotalBlocks:  0,
		DeadBlocks:   0,
		AnalysisTime: time.Since(startTime),
	}

	// Handle nil or empty CFG
	if dcd.cfg == nil || dcd.cfg.Blocks == nil {
		return result
	}

	result.TotalBlocks = len(dcd.cfg.Blocks)

	classifier := javaScriptCFGClassifier{}
	reachResult := corecfg.AnalyzeReachability(dcd.cfg, corecfg.ReachabilityConfig{Classifier: classifier})
	if result.TotalBlocks > 0 {
		result.ReachableRatio = float64(reachResult.ReachableCount) / float64(result.TotalBlocks)
	}
	coreResult := corecfg.DetectDeadCode(dcd.cfg, corecfg.DeadCodeConfig{Classifier: classifier})

	for _, coreFinding := range coreResult.Findings {
		block := dcd.cfg.GetBlock(coreFinding.BlockID)
		if block == nil || len(block.Statements) == 0 {
			continue
		}
		findings := dcd.analyzeDeadBlock(block)
		result.Findings = append(result.Findings, findings...)
	}
	result.DeadBlocks = len(result.Findings)

	// Merge overlapping/contiguous findings that share a reason. A compound
	// statement (e.g. `if`) spans its body, so the body's own block produces a
	// finding whose line range is nested inside the `if` finding's range. Left
	// as-is, the same source line is reported—and tallied—more than once. Merging
	// collapses each contiguous dead region into a single non-overlapping finding.
	result.Findings = mergeContiguousFindings(result.Findings)

	result.AnalysisTime = time.Since(startTime)
	return result
}

// analyzeDeadBlock analyzes a dead block to determine the reason and create findings
func (dcd *DeadCodeDetector) analyzeDeadBlock(block *BasicBlock) []*DeadCodeFinding {
	var findings []*DeadCodeFinding

	// Skip blocks whose only "statements" are empty separators (a bare `;`).
	// A trailing semicolon (`return y;;`) parses as the terminating statement
	// followed by an empty statement. That empty statement is technically
	// unreachable, but reporting it is noise — there's nothing for the user to
	// act on beyond a stylistic extra semicolon.
	if isOnlyEmptyStatements(block) {
		return findings
	}

	// Determine the reason for unreachability
	reason, severity := dcd.determineDeadCodeReason(block)

	// Create a finding for the block
	finding := &DeadCodeFinding{
		FunctionName: dcd.getFunctionName(),
		FilePath:     dcd.getFilePath(),
		BlockID:      block.ID,
		Reason:       reason,
		Severity:     severity,
		Description:  dcd.generateDescription(reason),
	}

	// Extract location from first statement in block
	if len(block.Statements) > 0 {
		firstStmt, firstOK := jsNode(block.Statements[0])
		lastStmt, lastOK := jsNode(block.Statements[len(block.Statements)-1])
		if firstOK {
			finding.StartLine = firstStmt.Location.StartLine
		}
		if lastOK {
			finding.EndLine = lastStmt.Location.EndLine
		}

		// Generate code snippet
		finding.Code = dcd.getCodeSnippet(block.Statements)
	}

	findings = append(findings, finding)
	return findings
}

// determineDeadCodeReason determines why a block is unreachable
func (dcd *DeadCodeDetector) determineDeadCodeReason(block *BasicBlock) (DeadCodeReason, SeverityLevel) {
	// Check predecessors for terminating statements
	for _, pred := range block.Predecessors {
		if pred.From == nil {
			continue
		}

		// Check last statement in predecessor block
		if len(pred.From.Statements) > 0 {
			lastStmt, ok := jsNode(pred.From.Statements[len(pred.From.Statements)-1])
			if !ok {
				continue
			}

			switch lastStmt.Type {
			case parser.NodeReturnStatement:
				return ReasonUnreachableAfterReturn, SeverityLevelCritical
			case parser.NodeBreakStatement:
				return ReasonUnreachableAfterBreak, SeverityLevelCritical
			case parser.NodeContinueStatement:
				return ReasonUnreachableAfterContinue, SeverityLevelCritical
			case parser.NodeThrowStatement:
				return ReasonUnreachableAfterThrow, SeverityLevelCritical
			}
		}
	}

	switch {
	case strings.Contains(block.ID, LabelUnreachableAfterReturn):
		return ReasonUnreachableAfterReturn, SeverityLevelCritical
	case strings.Contains(block.ID, LabelUnreachableAfterBreak):
		return ReasonUnreachableAfterBreak, SeverityLevelCritical
	case strings.Contains(block.ID, LabelUnreachableAfterContinue):
		return ReasonUnreachableAfterContinue, SeverityLevelCritical
	case strings.Contains(block.ID, LabelUnreachableAfterThrow):
		return ReasonUnreachableAfterThrow, SeverityLevelCritical
	case strings.Contains(block.ID, LabelUnreachable):
		return ReasonUnreachableAfterInfiniteLoop, SeverityLevelWarning
	}

	// Default to unreachable branch
	return ReasonUnreachableBranch, SeverityLevelWarning
}

// generateDescription generates a human-readable description for a dead code reason
func (dcd *DeadCodeDetector) generateDescription(reason DeadCodeReason) string {
	descriptions := map[DeadCodeReason]string{
		ReasonUnreachableAfterReturn:       "Code after return statement is unreachable",
		ReasonUnreachableAfterBreak:        "Code after break statement is unreachable",
		ReasonUnreachableAfterContinue:     "Code after continue statement is unreachable",
		ReasonUnreachableAfterThrow:        "Code after throw statement is unreachable",
		ReasonUnreachableBranch:            "This branch is unreachable",
		ReasonUnreachableAfterInfiniteLoop: "Code after infinite loop is unreachable",
		ReasonUnusedImport:                 "Imported name is never used in this file",
		ReasonUnusedExport:                 "Exported name is not imported by any other analyzed file",
		ReasonOrphanFile:                   "File is not imported by any other analyzed file",
		ReasonUnusedExportedFunction:       "Exported function is not imported by any other analyzed file",
	}

	if desc, exists := descriptions[reason]; exists {
		return desc
	}
	return "Code is unreachable"
}

// getCodeSnippet generates a code snippet from statements
func (dcd *DeadCodeDetector) getCodeSnippet(statements []any) string {
	if len(statements) == 0 {
		return ""
	}

	var snippets []string
	for _, value := range statements {
		stmt, ok := jsNode(value)
		if !ok {
			continue
		}
		// Use a simplified representation for now
		snippets = append(snippets, string(stmt.Type))
	}

	snippet := strings.Join(snippets, "; ")
	if len(snippet) > 100 {
		snippet = snippet[:100] + "..."
	}

	return snippet
}

// getFunctionName returns the function name from the CFG
func (dcd *DeadCodeDetector) getFunctionName() string {
	if dcd.cfg != nil {
		return dcd.cfg.Name
	}
	return ""
}

// getFilePath returns the file path for context
func (dcd *DeadCodeDetector) getFilePath() string {
	return dcd.filePath
}

// DetectAll analyzes dead code for all functions in a file
func DetectAll(cfgs map[string]*CFG, filePath string) map[string]*DeadCodeResult {
	results := make(map[string]*DeadCodeResult)

	for name, cfg := range cfgs {
		detector := NewDeadCodeDetectorWithFilePath(cfg, filePath)
		result := detector.Detect()
		results[name] = result
	}

	return results
}

// mergeContiguousFindings collapses findings whose line ranges overlap or are
// directly adjacent (no reachable line between them) and that share the same
// reason into a single finding. Findings must be pre-sorted by StartLine (then
// EndLine). This removes the overlapping/duplicate ranges that arise because a
// compound statement's finding spans its body while the body's block emits its
// own nested finding.
func mergeContiguousFindings(findings []*DeadCodeFinding) []*DeadCodeFinding {
	lineFindings := make([]*corecfg.LineFinding, 0, len(findings))
	origins := make(map[*corecfg.LineFinding]*DeadCodeFinding, len(findings))
	for _, finding := range findings {
		lineFinding := &corecfg.LineFinding{
			StartLine:   finding.StartLine,
			EndLine:     finding.EndLine,
			Reason:      string(finding.Reason),
			Severity:    toCoreSeverity(finding.Severity),
			Description: finding.Description,
			Code:        finding.Code,
		}
		lineFindings = append(lineFindings, lineFinding)
		origins[lineFinding] = finding
	}
	corecfg.SortLineFindings(lineFindings)
	mergedLines := corecfg.MergeContiguousFindings(lineFindings)
	merged := make([]*DeadCodeFinding, 0, len(mergedLines))
	for _, lineFinding := range mergedLines {
		finding := origins[lineFinding]
		finding.StartLine = lineFinding.StartLine
		finding.EndLine = lineFinding.EndLine
		finding.Severity = fromCoreSeverity(lineFinding.Severity)
		finding.Description = lineFinding.Description
		finding.Code = lineFinding.Code
		merged = append(merged, finding)
	}
	return merged
}

// isOnlyEmptyStatements reports whether every statement in the block is an
// empty separator node (a bare `;`). Such blocks are unreachable but carry no
// actionable signal, so they should not produce dead-code findings.
func isOnlyEmptyStatements(block *BasicBlock) bool {
	if block == nil || len(block.Statements) == 0 {
		return false
	}
	classifier := javaScriptCFGClassifier{}
	for _, stmt := range block.Statements {
		if !classifier.IsNoOp(stmt) {
			return false
		}
	}
	return true
}

func toCoreSeverity(severity SeverityLevel) corecfg.DeadCodeSeverity {
	switch severity {
	case SeverityLevelCritical:
		return corecfg.SeverityCritical
	case SeverityLevelWarning:
		return corecfg.SeverityWarning
	default:
		return corecfg.SeverityInfo
	}
}

func fromCoreSeverity(severity corecfg.DeadCodeSeverity) SeverityLevel {
	switch severity {
	case corecfg.SeverityCritical:
		return SeverityLevelCritical
	case corecfg.SeverityWarning:
		return SeverityLevelWarning
	default:
		return SeverityLevelInfo
	}
}

// HasFindings returns true if there are any dead code findings
func (dcr *DeadCodeResult) HasFindings() bool {
	return len(dcr.Findings) > 0
}

// GetCriticalFindings returns only critical severity findings
func (dcr *DeadCodeResult) GetCriticalFindings() []*DeadCodeFinding {
	var critical []*DeadCodeFinding
	for _, finding := range dcr.Findings {
		if finding.Severity == SeverityLevelCritical {
			critical = append(critical, finding)
		}
	}
	return critical
}

// GetWarningFindings returns only warning severity findings
func (dcr *DeadCodeResult) GetWarningFindings() []*DeadCodeFinding {
	var warnings []*DeadCodeFinding
	for _, finding := range dcr.Findings {
		if finding.Severity == SeverityLevelWarning {
			warnings = append(warnings, finding)
		}
	}
	return warnings
}
