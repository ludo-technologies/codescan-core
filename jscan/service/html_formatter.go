package service

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
)

// HTMLData represents the data for HTML template
type HTMLData struct {
	GeneratedAt      string
	Duration         int64
	Version          string
	Complexity       *domain.ComplexityResponse
	DeadCode         *domain.DeadCodeResponse
	Clone            *domain.CloneResponse
	CBO              *domain.CBOResponse
	Deps             *domain.DependencyGraphResponse
	ModuleQuality    []domain.ModuleQualityMetrics
	Summary          *domain.AnalyzeSummary
	HasComplexity    bool
	HasDeadCode      bool
	HasClone         bool
	HasCBO           bool
	HasDeps          bool
	HasModuleQuality bool
}

// WriteHTML writes the analysis result as HTML
func (f *OutputFormatterImpl) WriteHTML(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
	writer io.Writer,
	duration time.Duration,
) error {
	now := time.Now()

	if cloneResponse != nil {
		if cloneResponse.Statistics == nil {
			cloneResponse.Statistics = &domain.CloneStatistics{}
		}
		clonePairs := make([]*domain.ClonePair, 0, len(cloneResponse.ClonePairs))
		for _, pair := range cloneResponse.ClonePairs {
			if pair != nil {
				clonePairs = append(clonePairs, pair)
			}
		}
		cloneResponse.ClonePairs = clonePairs
	}

	// Build summary (reuse shared logic to avoid score divergence across output formats)
	summary := BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)
	moduleQuality := BuildModuleQuality(complexityResponse, deadCodeResponse, depsResponse)

	data := HTMLData{
		GeneratedAt:      now.Format("2006-01-02 15:04:05"),
		Duration:         duration.Milliseconds(),
		Version:          version.Version,
		Complexity:       complexityResponse,
		DeadCode:         deadCodeResponse,
		Clone:            cloneResponse,
		CBO:              cboResponse,
		Deps:             depsResponse,
		ModuleQuality:    moduleQuality,
		Summary:          summary,
		HasComplexity:    complexityResponse != nil,
		HasDeadCode:      deadCodeResponse != nil,
		HasClone:         cloneResponse != nil,
		HasCBO:           cboResponse != nil,
		HasDeps:          depsResponse != nil,
		HasModuleQuality: len(moduleQuality) > 0,
	}

	funcMap := template.FuncMap{
		"join": func(elems []string, sep string) string {
			return strings.Join(elems, sep)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"cloneLoc": func(clone *domain.Clone) string {
			if clone == nil || clone.Location == nil {
				return "unknown"
			}
			return fmt.Sprintf("%s:%d", clone.Location.FilePath, clone.Location.StartLine)
		},
		"scoreQuality": func(score int) string {
			switch {
			case score >= domain.ScoreThresholdExcellent:
				return "excellent"
			case score >= domain.ScoreThresholdGood:
				return "good"
			case score >= domain.ScoreThresholdFair:
				return "fair"
			default:
				return "poor"
			}
		},
		"projectScale": func(summary *domain.AnalyzeSummary) string {
			if summary == nil {
				return ""
			}
			return FormatProjectScale(summary)
		},
		"gradeClass": func(grade string) string {
			switch grade {
			case "A":
				return "grade-a"
			case "B":
				return "grade-b"
			case "C":
				return "grade-c"
			case "D":
				return "grade-d"
			default:
				return "grade-f"
			}
		},
	}

	tmpl := template.Must(template.New("analyze").Funcs(funcMap).Parse(htmlTemplate))
	return tmpl.Execute(writer, data)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>jscan Analysis Report</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: white;
            border-radius: 10px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
        }
        .header h1 {
            color: #667eea;
            margin-bottom: 10px;
        }
        .header .subtitle {
            color: #666;
            font-size: 14px;
        }
        .score-badge {
            display: inline-block;
            padding: 10px 20px;
            border-radius: 50px;
            font-size: 24px;
            font-weight: bold;
            margin: 10px 0;
        }
        .grade-a { background: #4caf50; color: white; }
        .grade-b { background: #8bc34a; color: white; }
        .grade-c { background: #ff9800; color: white; }
        .grade-d { background: #ff5722; color: white; }
        .grade-f { background: #f44336; color: white; }

        .tabs {
            background: white;
            border-radius: 10px;
            overflow: hidden;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
        }
        .tab-buttons {
            display: flex;
            background: #f5f5f5;
        }
        .tab-button {
            flex: 1;
            padding: 15px;
            border: none;
            background: transparent;
            cursor: pointer;
            font-size: 16px;
            transition: all 0.3s;
        }
        .tab-button.active {
            background: white;
            color: #667eea;
            font-weight: bold;
        }
        .tab-content {
            display: none;
            padding: 30px;
        }
        .tab-content.active {
            display: block;
        }

        .metric-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }
        .metric-card {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
        }
        .metric-value {
            font-size: 32px;
            font-weight: bold;
            color: #667eea;
        }
        .metric-label {
            color: #666;
            margin-top: 5px;
        }

        .table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        .table th, .table td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        .table th {
            background: #f8f9fa;
            font-weight: 600;
        }

        .risk-low { color: #4caf50; }
        .risk-medium { color: #ff9800; }
        .risk-high { color: #f44336; }

        .severity-critical { color: #f44336; }
        .severity-warning { color: #ff9800; }
        .severity-info { color: #2196f3; }

        .score-bars {
            margin: 20px 0;
        }
        .score-bar-item {
            margin-bottom: 24px;
        }
        .score-bar-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 6px;
            font-size: 14px;
        }
        .score-label {
            font-weight: 600;
            color: #333;
        }
        .score-value {
            font-weight: 700;
            color: #667eea;
        }
        .score-bar-container {
            width: 100%;
            height: 12px;
            background: #e0e0e0;
            border-radius: 6px;
            overflow: hidden;
        }
        .score-bar-fill {
            height: 100%;
            transition: width 0.3s ease;
            border-radius: 6px;
        }
        .score-excellent { background: linear-gradient(90deg, #4caf50, #66bb6a); }
        .score-good { background: linear-gradient(90deg, #8bc34a, #9ccc65); }
        .score-fair { background: linear-gradient(90deg, #ff9800, #ffa726); }
        .score-poor { background: linear-gradient(90deg, #f44336, #ef5350); }
        .score-detail {
            margin-top: 4px;
            font-size: 12px;
            color: #666;
        }

        .tab-header-with-score {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 20px;
            padding-bottom: 12px;
            border-bottom: 2px solid #e0e0e0;
        }

        .score-badge-compact {
            display: inline-block;
            padding: 6px 14px;
            border-radius: 16px;
            font-size: 13px;
            font-weight: 700;
            color: white;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>jscan Analysis Report</h1>
            <p class="subtitle">Generated: {{.GeneratedAt}} | Duration: {{.Duration}}ms | Version: {{.Version}}</p>
            <div class="score-badge {{gradeClass .Summary.Grade}}">
                Health Score: {{.Summary.HealthScore}}/100 (Grade: {{.Summary.Grade}})
            </div>
            <p class="subtitle">Project Scale: {{projectScale .Summary}}</p>
        </div>

        <div class="tabs">
            <div class="tab-buttons">
                <button class="tab-button active" onclick="showTab('summary', this)">Summary</button>
                {{if .HasComplexity}}
                <button class="tab-button" onclick="showTab('complexity', this)">Complexity</button>
                {{end}}
                {{if .HasDeadCode}}
                <button class="tab-button" onclick="showTab('deadcode', this)">Dead Code</button>
                {{end}}
                {{if .HasClone}}
                <button class="tab-button" onclick="showTab('clone', this)">Clones</button>
                {{end}}
                {{if .HasCBO}}
                <button class="tab-button" onclick="showTab('cbo', this)">Coupling</button>
                {{end}}
                {{if .HasDeps}}
                <button class="tab-button" onclick="showTab('deps', this)">Dependencies</button>
                {{end}}
                {{if .HasModuleQuality}}
                <button class="tab-button" onclick="showTab('modules', this)">Modules</button>
                {{end}}
            </div>

            <div id="summary" class="tab-content active">
                <h2>Analysis Summary</h2>

                <h3 style="margin-top: 20px; margin-bottom: 16px; color: #2c3e50;">Quality Scores</h3>
                <div class="score-bars">
                    {{if .HasComplexity}}
                    <div class="score-bar-item">
                        <div class="score-bar-header">
                            <span class="score-label">Complexity</span>
                            <span class="score-value">{{.Summary.ComplexityScore}}/100</span>
                        </div>
                        <div class="score-bar-container">
                            <div class="score-bar-fill score-{{scoreQuality .Summary.ComplexityScore}}" style="width: {{.Summary.ComplexityScore}}%"></div>
                        </div>
                        <div class="score-detail">Avg: {{printf "%.1f" .Summary.AverageComplexity}}, High-risk: {{.Summary.HighComplexityCount}}</div>
                    </div>
                    {{end}}

                    {{if .HasDeadCode}}
                    <div class="score-bar-item">
                        <div class="score-bar-header">
                            <span class="score-label">Dead Code</span>
                            <span class="score-value">{{.Summary.DeadCodeScore}}/100</span>
                        </div>
                        <div class="score-bar-container">
                            <div class="score-bar-fill score-{{scoreQuality .Summary.DeadCodeScore}}" style="width: {{.Summary.DeadCodeScore}}%"></div>
                        </div>
                        <div class="score-detail">{{.Summary.DeadCodeCount}} issues, {{.Summary.CriticalDeadCode}} critical</div>
                    </div>
                    {{end}}

                    {{if .HasClone}}
                    <div class="score-bar-item">
                        <div class="score-bar-header">
                            <span class="score-label">Code Duplication</span>
                            <span class="score-value">{{.Summary.DuplicationScore}}/100</span>
                        </div>
                        <div class="score-bar-container">
                            <div class="score-bar-fill score-{{scoreQuality .Summary.DuplicationScore}}" style="width: {{.Summary.DuplicationScore}}%"></div>
                        </div>
                        <div class="score-detail">{{.Summary.ClonePairs}} clone pairs, {{printf "%.1f" .Summary.CodeDuplication}}% duplication</div>
                    </div>
                    {{end}}

                    {{if .HasCBO}}
                    <div class="score-bar-item">
                        <div class="score-bar-header">
                            <span class="score-label">Coupling</span>
                            <span class="score-value">{{.Summary.CouplingScore}}/100</span>
                        </div>
                        <div class="score-bar-container">
                            <div class="score-bar-fill score-{{scoreQuality .Summary.CouplingScore}}" style="width: {{.Summary.CouplingScore}}%"></div>
                        </div>
                        <div class="score-detail">{{.Summary.HighCouplingClasses}} high-risk classes, avg CBO: {{printf "%.1f" .Summary.AverageCoupling}}</div>
                    </div>
                    {{end}}

                    {{if .HasDeps}}
                    <div class="score-bar-item">
                        <div class="score-bar-header">
                            <span class="score-label">Dependencies</span>
                            <span class="score-value">{{.Summary.DependencyScore}}/100</span>
                        </div>
                        <div class="score-bar-container">
                            <div class="score-bar-fill score-{{scoreQuality .Summary.DependencyScore}}" style="width: {{.Summary.DependencyScore}}%"></div>
                        </div>
                        <div class="score-detail">{{.Summary.DepsTotalModules}} modules, {{.Summary.DepsModulesInCycles}} in cycles</div>
                    </div>
                    {{end}}
                </div>

                <h3 style="margin-top: 24px; margin-bottom: 16px; color: #2c3e50;">File Statistics</h3>
                <div class="metric-grid">
                    <div class="metric-card">
                        <div class="metric-value">{{.Summary.AnalyzedFiles}}</div>
                        <div class="metric-label">Files Analyzed</div>
                    </div>
                    {{if .HasComplexity}}
                    <div class="metric-card">
                        <div class="metric-value">{{.Summary.TotalFunctions}}</div>
                        <div class="metric-label">Total Functions</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{printf "%.2f" .Summary.AverageComplexity}}</div>
                        <div class="metric-label">Avg Complexity</div>
                    </div>
                    {{end}}
                    {{if .HasDeadCode}}
                    <div class="metric-card">
                        <div class="metric-value">{{.Summary.DeadCodeCount}}</div>
                        <div class="metric-label">Dead Code Issues</div>
                    </div>
                    {{end}}
                </div>
            </div>

            {{if .HasComplexity}}
            <div id="complexity" class="tab-content">
                <div class="tab-header-with-score">
                    <h2 style="margin: 0;">Complexity Analysis</h2>
                    <div class="score-badge-compact score-{{scoreQuality .Summary.ComplexityScore}}">
                        {{.Summary.ComplexityScore}}/100
                    </div>
                </div>

                <div class="metric-grid">
                    <div class="metric-card">
                        <div class="metric-value">{{.Complexity.Summary.TotalFunctions}}</div>
                        <div class="metric-label">Total Functions</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{printf "%.2f" .Complexity.Summary.AverageComplexity}}</div>
                        <div class="metric-label">Average</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.Complexity.Summary.MaxComplexity}}</div>
                        <div class="metric-label">Maximum</div>
                    </div>
                </div>

                {{if .Complexity.ByDirectory}}
                <h3>Directory Complexity</h3>
                <div style="overflow-x: auto;">
                    <table class="table">
                        <thead>
                            <tr>
                                <th>Directory</th>
                                <th>Functions</th>
                                <th>Avg CC</th>
                                <th>Max CC</th>
                                <th>High Risk</th>
                                <th>Avg Nesting</th>
                                <th>Max Nesting</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Complexity.ByDirectory}}
                            <tr>
                                <td>{{.DirectoryPath}}</td>
                                <td>{{.FunctionCount}}</td>
                                <td>{{printf "%.2f" .AverageComplexity}}</td>
                                <td>{{.MaxComplexity}}</td>
                                <td>{{.HighRiskFunctionCount}}</td>
                                <td>{{printf "%.2f" .AverageNestingDepth}}</td>
                                <td>{{.MaxNestingDepth}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
                {{end}}

                <h3>Functions</h3>
                <table class="table">
                    <thead>
                        <tr>
                            <th>Function</th>
                            <th>File</th>
                            <th>Complexity</th>
                            <th>Risk</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $i, $f := .Complexity.Functions}}
                        {{if lt $i 20}}
                        <tr>
                            <td>{{$f.Name}}</td>
                            <td>{{$f.FilePath}}</td>
                            <td>{{$f.Metrics.Complexity}}</td>
                            <td class="risk-{{$f.RiskLevel}}">{{$f.RiskLevel}}</td>
                        </tr>
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
                {{if gt (len .Complexity.Functions) 20}}
                <p style="color: #666; margin-top: 10px;">Showing top 20 of {{len .Complexity.Functions}} functions</p>
                {{end}}
            </div>
            {{end}}

            {{if .HasDeadCode}}
            <div id="deadcode" class="tab-content">
                <div class="tab-header-with-score">
                    <h2 style="margin: 0;">Dead Code Detection</h2>
                    <div class="score-badge-compact score-{{scoreQuality .Summary.DeadCodeScore}}">
                        {{.Summary.DeadCodeScore}}/100
                    </div>
                </div>

                <div class="metric-grid">
                    <div class="metric-card">
                        <div class="metric-value">{{.DeadCode.Summary.TotalFindings}}</div>
                        <div class="metric-label">Total Issues</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.DeadCode.Summary.CriticalFindings}}</div>
                        <div class="metric-label">Critical</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.DeadCode.Summary.WarningFindings}}</div>
                        <div class="metric-label">Warnings</div>
                    </div>
                </div>

                {{if gt .DeadCode.Summary.TotalFindings 0}}
                <h3>Dead Code Issues</h3>
                <table class="table">
                    <thead>
                        <tr>
                            <th>File</th>
                            <th>Function</th>
                            <th>Lines</th>
                            <th>Severity</th>
                            <th>Reason</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $file := .DeadCode.Files}}
                        {{range $finding := $file.FileLevelFindings}}
                        <tr>
                            <td>{{$finding.Location.FilePath}}</td>
                            <td><em>&lt;file-level&gt;</em></td>
                            <td>{{$finding.Location.StartLine}}-{{$finding.Location.EndLine}}</td>
                            <td class="severity-{{$finding.Severity}}">{{$finding.Severity}}</td>
                            <td>{{$finding.Description}}</td>
                        </tr>
                        {{end}}
                        {{range $func := $file.Functions}}
                        {{range $i, $finding := $func.Findings}}
                        {{if lt $i 20}}
                        <tr>
                            <td>{{$finding.Location.FilePath}}</td>
                            <td>{{$finding.FunctionName}}</td>
                            <td>{{$finding.Location.StartLine}}-{{$finding.Location.EndLine}}</td>
                            <td class="severity-{{$finding.Severity}}">{{$finding.Severity}}</td>
                            <td>{{$finding.Reason}}</td>
                        </tr>
                        {{end}}
                        {{end}}
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
                {{else}}
                <p style="color: #4caf50; font-weight: bold; margin-top: 20px;">✓ No dead code detected</p>
                {{end}}
            </div>
            {{end}}

            {{if .HasClone}}
            <div id="clone" class="tab-content">
                <div class="tab-header-with-score">
                    <h2 style="margin: 0;">Clone Detection</h2>
                    <div class="score-badge-compact score-{{scoreQuality .Summary.DuplicationScore}}">
                        {{.Summary.DuplicationScore}}/100
                    </div>
                </div>

                <div class="metric-grid">
                    <div class="metric-card">
                        <div class="metric-value">{{.Clone.Statistics.TotalClonePairs}}</div>
                        <div class="metric-label">Clone Pairs</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.Clone.Statistics.TotalCloneGroups}}</div>
                        <div class="metric-label">Clone Groups</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{printf "%.1f%%" .Summary.CodeDuplication}}</div>
                        <div class="metric-label">Code Duplication</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{printf "%.0f%%" (mul .Clone.Statistics.AverageSimilarity 100)}}</div>
                        <div class="metric-label">Avg Similarity</div>
                    </div>
                </div>

                {{if gt .Clone.Statistics.TotalClonePairs 0}}
                <h3>Top Clone Pairs</h3>
                <table class="table">
                    <thead>
                        <tr>
                            <th>Type</th>
                            <th>Location 1</th>
                            <th>Location 2</th>
                            <th>Similarity</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $i, $pair := .Clone.ClonePairs}}
                        {{if lt $i 20}}
                        <tr>
                            <td>{{$pair.Type}}</td>
                            <td>{{cloneLoc $pair.Clone1}}</td>
                            <td>{{cloneLoc $pair.Clone2}}</td>
                            <td>{{printf "%.1f%%" (mul $pair.Similarity 100)}}</td>
                        </tr>
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
                {{if gt (len .Clone.ClonePairs) 20}}
                <p style="color: #666; margin-top: 10px;">Showing top 20 of {{len .Clone.ClonePairs}} clone pairs</p>
                {{end}}
                {{else}}
                <p style="color: #4caf50; font-weight: bold; margin-top: 20px;">✓ No code clones detected</p>
                {{end}}
            </div>
            {{end}}

            {{if .HasCBO}}
            <div id="cbo" class="tab-content">
                <div class="tab-header-with-score">
                    <h2 style="margin: 0;">Coupling Analysis (CBO)</h2>
                    <div class="score-badge-compact score-{{scoreQuality .Summary.CouplingScore}}">
                        {{.Summary.CouplingScore}}/100
                    </div>
                </div>

                <div class="metric-grid">
                    <div class="metric-card">
                        <div class="metric-value">{{.CBO.Summary.TotalClasses}}</div>
                        <div class="metric-label">Classes Analyzed</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{printf "%.2f" .CBO.Summary.AverageCBO}}</div>
                        <div class="metric-label">Average CBO</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.CBO.Summary.HighRiskClasses}}</div>
                        <div class="metric-label">High Coupling</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.CBO.Summary.MediumRiskClasses}}</div>
                        <div class="metric-label">Medium Coupling</div>
                    </div>
                </div>

                {{if gt .CBO.Summary.TotalClasses 0}}
                <h3>Classes by Coupling</h3>
                <table class="table">
                    <thead>
                        <tr>
                            <th>Class</th>
                            <th>File</th>
                            <th>CBO</th>
                            <th>Risk</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $i, $class := .CBO.Classes}}
                        {{if lt $i 20}}
                        <tr>
                            <td>{{$class.Name}}</td>
                            <td>{{$class.FilePath}}</td>
                            <td>{{$class.Metrics.CouplingCount}}</td>
                            <td class="risk-{{$class.RiskLevel}}">{{$class.RiskLevel}}</td>
                        </tr>
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
                {{if gt (len .CBO.Classes) 20}}
                <p style="color: #666; margin-top: 10px;">Showing top 20 of {{len .CBO.Classes}} classes</p>
                {{end}}
                {{else}}
                <p style="color: #666; margin-top: 20px;">No classes found for CBO analysis</p>
                {{end}}
            </div>
            {{end}}

            {{if .HasDeps}}
            <div id="deps" class="tab-content">
                <div class="tab-header-with-score">
                    <h2 style="margin: 0;">Dependency Analysis</h2>
                    <div class="score-badge-compact score-{{scoreQuality .Summary.DependencyScore}}">
                        {{.Summary.DependencyScore}}/100
                    </div>
                </div>

                <div class="metric-grid">
                    <div class="metric-card">
                        <div class="metric-value">{{.Summary.DepsTotalModules}}</div>
                        <div class="metric-label">Total Modules</div>
                    </div>
                    {{if .Deps.Analysis}}
                    <div class="metric-card">
                        <div class="metric-value">{{len .Deps.Analysis.RootModules}}</div>
                        <div class="metric-label">Entry Points</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.Deps.Analysis.MaxDepth}}</div>
                        <div class="metric-label">Max Depth</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-value">{{.Summary.DepsModulesInCycles}}</div>
                        <div class="metric-label">In Cycles</div>
                    </div>
                    {{end}}
                </div>

                {{if .Deps.Analysis}}
                {{if .Deps.Analysis.CircularDependencies}}
                {{if .Deps.Analysis.CircularDependencies.HasCircularDependencies}}
                <h3 style="color: #f44336;">Circular Dependencies</h3>
                <table class="table">
                    <thead>
                        <tr>
                            <th>#</th>
                            <th>Severity</th>
                            <th>Modules in Cycle</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $i, $cycle := .Deps.Analysis.CircularDependencies.CircularDependencies}}
                        {{if lt $i 10}}
                        <tr>
                            <td>{{add $i 1}}</td>
                            <td class="severity-{{$cycle.Severity}}">{{$cycle.Severity}}</td>
                            <td>{{join $cycle.Modules " → "}}</td>
                        </tr>
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
                {{if gt (len .Deps.Analysis.CircularDependencies.CircularDependencies) 10}}
                <p style="color: #666; margin-top: 10px;">Showing 10 of {{len .Deps.Analysis.CircularDependencies.CircularDependencies}} cycles</p>
                {{end}}
                {{else}}
                <p style="color: #4caf50; font-weight: bold; margin-top: 20px;">✓ No circular dependencies detected</p>
                {{end}}
                {{end}}
                {{end}}
            </div>
            {{end}}

            {{if .HasModuleQuality}}
            <div id="modules" class="tab-content">
                <h2>Module Quality Hotspots</h2>
                <p style="color: #666; margin-bottom: 20px;">Per-module metrics ranked by high-risk functions, maximum complexity, average complexity, and dead-code findings</p>
                <div style="overflow-x: auto;">
                    <table class="table">
                        <thead>
                            <tr>
                                <th>Module</th>
                                <th>File</th>
                                <th>LOC</th>
                                <th>Analyzed</th>
                                <th>Avg CC</th>
                                <th>Max CC</th>
                                <th>High Risk</th>
                                <th>Handlers</th>
                                <th>Dead Findings</th>
                                <th>Dead Blocks</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range $i, $module := .ModuleQuality}}
                            {{if lt $i 20}}
                            <tr>
                                <td>{{if $module.ModuleName}}{{$module.ModuleName}}{{else}}&mdash;{{end}}</td>
                                <td>{{$module.FilePath}}</td>
                                <td>{{$module.LinesOfCode}}</td>
                                <td>{{$module.AnalyzedFunctionCount}}</td>
                                <td>{{printf "%.2f" $module.AverageComplexity}}</td>
                                <td>{{$module.MaxComplexity}}</td>
                                <td>{{$module.HighRiskFunctionCount}}</td>
                                <td>{{$module.ExceptionHandlerCount}}</td>
                                <td>{{$module.DeadCodeFindingCount}}</td>
                                <td>{{$module.DeadCodeBlockCount}}</td>
                            </tr>
                            {{end}}
                            {{end}}
                        </tbody>
                    </table>
                </div>
                {{if gt (len .ModuleQuality) 20}}
                <p style="color: #666; margin-top: 10px;">Showing top 20 of {{len .ModuleQuality}} modules</p>
                {{end}}
            </div>
            {{end}}
        </div>
    </div>

    <script>
        function showTab(tabName, el) {
            const tabs = document.querySelectorAll('.tab-content');
            tabs.forEach(tab => tab.classList.remove('active'));

            const buttons = document.querySelectorAll('.tab-button');
            buttons.forEach(btn => btn.classList.remove('active'));

            document.getElementById(tabName).classList.add('active');
            if (el) { el.classList.add('active'); }
        }
    </script>
</body>
</html>`
