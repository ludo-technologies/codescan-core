// Package domain holds jscan's models and the interfaces its services
// implement. It is the innermost layer: every other package may import it,
// and it imports none of them.
//
// The types here fall into three groups.
//
// Request and response pairs describe one analysis each. ComplexityRequest and
// ComplexityResponse, DeadCodeRequest and DeadCodeResponse, CloneRequest and
// CloneResponse, CBORequest and CBOResponse, and DependencyGraphRequest and
// DependencyGraphResponse follow the same shape: the request carries paths and
// thresholds, and the response carries findings, a summary, and any warnings or
// errors gathered along the way. A failure analyzing one file is reported in
// those slices rather than returned as an error, so a single unparsable file
// never costs the caller the whole run.
//
// AnalyzeSummary aggregates all five into the health score. Its
// CalculateHealthScore method applies the penalty model from
// polyscan/core/domain, which pyscn shares, so a grade means the same thing in
// both analyzers.
//
// Adapters connect the language-neutral algorithms in polyscan/core to jscan's
// own types. Clone satisfies core/clone.GroupableItem through ItemID and
// ItemLocation so that the shared grouping strategies can operate on JavaScript
// fragments, and DependencyGraph satisfies core/graph.DirectedGraph through
// NodeIDs, Successors, and Predecessors so that Tarjan cycle detection and the
// Martin coupling metrics can run over the module graph.
package domain
