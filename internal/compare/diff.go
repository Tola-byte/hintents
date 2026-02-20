// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package compare

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

const sideBySideColumnWidth = 72

var callPathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)invoke_contract`),
	regexp.MustCompile(`(?i)contract_call`),
	regexp.MustCompile(`(?i)call\(`),
	regexp.MustCompile(`(?i)invoke`),
}

// Diff represents differences between two simulation results.
type Diff struct {
	StatusChanged bool
	ErrorChanged  bool
	OnChainStatus string
	LocalStatus   string
	OnChainError  string
	LocalError    string
	EventsDiff    []EventDiff
	LogsDiff      []LogDiff
	BudgetDiff    BudgetDiff
	CallPathDiff  []CallPathDiff
	Summary       string
}

// EventDiff represents a difference in events.
type EventDiff struct {
	Index   int
	OnChain string
	Local   string
	Type    string // "added", "removed", "modified", "unchanged"
}

// LogDiff represents a difference in logs.
type LogDiff struct {
	Index   int
	OnChain string
	Local   string
	Type    string // "added", "removed", "modified", "unchanged"
}

// BudgetDiff captures local-vs-on-chain budget changes.
type BudgetDiff struct {
	Available bool
	OnChain   *simulator.BudgetUsage
	Local     *simulator.BudgetUsage
	CPUOps    int64
	Memory    int64
	OpsCount  int64
}

// CallPathDiff represents a divergence in extracted call path steps.
type CallPathDiff struct {
	Index   int
	OnChain string
	Local   string
	Type    string // "added", "removed", "modified", "unchanged"
}

// CompareResults compares two SimulationResponse objects and returns a Diff.
func CompareResults(onChain, local *simulator.SimulationResponse) *Diff {
	diff := &Diff{
		StatusChanged: onChain.Status != local.Status,
		ErrorChanged:  onChain.Error != local.Error,
		OnChainStatus: onChain.Status,
		LocalStatus:   local.Status,
		OnChainError:  onChain.Error,
		LocalError:    local.Error,
		EventsDiff:    compareEvents(onChain.Events, local.Events),
		LogsDiff:      compareLogs(onChain.Logs, local.Logs),
		BudgetDiff:    compareBudget(onChain.BudgetUsage, local.BudgetUsage),
		CallPathDiff:  compareCallPaths(extractCallPath(onChain), extractCallPath(local)),
	}

	diff.Summary = buildSummary(diff)
	return diff
}

func compareEvents(onChain, local []string) []EventDiff {
	var diffs []EventDiff
	maxLen := len(onChain)
	if len(local) > maxLen {
		maxLen = len(local)
	}

	for i := 0; i < maxLen; i++ {
		var ed EventDiff
		ed.Index = i

		if i < len(onChain) && i < len(local) {
			ed.OnChain = onChain[i]
			ed.Local = local[i]
			if onChain[i] == local[i] {
				ed.Type = "unchanged"
			} else {
				ed.Type = "modified"
			}
		} else if i < len(onChain) {
			ed.OnChain = onChain[i]
			ed.Local = ""
			ed.Type = "removed"
		} else {
			ed.OnChain = ""
			ed.Local = local[i]
			ed.Type = "added"
		}

		diffs = append(diffs, ed)
	}

	return diffs
}

func compareLogs(onChain, local []string) []LogDiff {
	var diffs []LogDiff
	maxLen := len(onChain)
	if len(local) > maxLen {
		maxLen = len(local)
	}

	for i := 0; i < maxLen; i++ {
		var ld LogDiff
		ld.Index = i

		if i < len(onChain) && i < len(local) {
			ld.OnChain = onChain[i]
			ld.Local = local[i]
			if onChain[i] == local[i] {
				ld.Type = "unchanged"
			} else {
				ld.Type = "modified"
			}
		} else if i < len(onChain) {
			ld.OnChain = onChain[i]
			ld.Local = ""
			ld.Type = "removed"
		} else {
			ld.OnChain = ""
			ld.Local = local[i]
			ld.Type = "added"
		}

		diffs = append(diffs, ld)
	}

	return diffs
}

func compareBudget(onChain, local *simulator.BudgetUsage) BudgetDiff {
	if onChain == nil || local == nil {
		return BudgetDiff{
			Available: false,
			OnChain:   onChain,
			Local:     local,
		}
	}

	return BudgetDiff{
		Available: true,
		OnChain:   onChain,
		Local:     local,
		CPUOps:    int64(local.CPUInstructions) - int64(onChain.CPUInstructions),
		Memory:    int64(local.MemoryBytes) - int64(onChain.MemoryBytes),
		OpsCount:  int64(local.OperationsCount) - int64(onChain.OperationsCount),
	}
}

func extractCallPath(resp *simulator.SimulationResponse) []string {
	var path []string

	for _, line := range resp.Events {
		normalized := normalizeCallPathLine(line)
		if normalized == "" {
			continue
		}
		if isCallPathLine(normalized) {
			path = append(path, normalized)
		}
	}

	for _, line := range resp.Logs {
		normalized := normalizeCallPathLine(line)
		if normalized == "" {
			continue
		}
		if isCallPathLine(normalized) {
			path = append(path, normalized)
		}
	}

	return path
}

func normalizeCallPathLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	return strings.Join(strings.Fields(line), " ")
}

func isCallPathLine(line string) bool {
	for _, pattern := range callPathPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func compareCallPaths(onChain, local []string) []CallPathDiff {
	var diffs []CallPathDiff
	maxLen := len(onChain)
	if len(local) > maxLen {
		maxLen = len(local)
	}

	for i := 0; i < maxLen; i++ {
		var d CallPathDiff
		d.Index = i
		if i < len(onChain) && i < len(local) {
			d.OnChain = onChain[i]
			d.Local = local[i]
			if onChain[i] == local[i] {
				d.Type = "unchanged"
			} else {
				d.Type = "modified"
			}
		} else if i < len(onChain) {
			d.OnChain = onChain[i]
			d.Type = "removed"
		} else {
			d.Local = local[i]
			d.Type = "added"
		}
		diffs = append(diffs, d)
	}

	return diffs
}

func buildSummary(diff *Diff) string {
	var parts []string

	if diff.StatusChanged {
		parts = append(parts, "status changed")
	}
	if diff.ErrorChanged {
		parts = append(parts, "error changed")
	}

	eventChanges := countEventChanges(diff.EventsDiff)
	if eventChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d event(s) differ", eventChanges))
	}

	logChanges := countLogChanges(diff.LogsDiff)
	if logChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d log(s) differ", logChanges))
	}

	callPathChanges := countCallPathChanges(diff.CallPathDiff)
	if callPathChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d call path step(s) diverged", callPathChanges))
	}

	if diff.BudgetDiff.Available && (diff.BudgetDiff.CPUOps != 0 || diff.BudgetDiff.Memory != 0 || diff.BudgetDiff.OpsCount != 0) {
		parts = append(parts, "budget usage changed")
	}

	if len(parts) == 0 {
		return "No differences found"
	}

	return strings.Join(parts, ", ")
}

func countEventChanges(diffs []EventDiff) int {
	count := 0
	for _, d := range diffs {
		if d.Type != "unchanged" {
			count++
		}
	}
	return count
}

func countLogChanges(diffs []LogDiff) int {
	count := 0
	for _, d := range diffs {
		if d.Type != "unchanged" {
			count++
		}
	}
	return count
}

func countCallPathChanges(diffs []CallPathDiff) int {
	count := 0
	for _, d := range diffs {
		if d.Type != "unchanged" {
			count++
		}
	}
	return count
}

func truncateForColumn(value string) string {
	if len(value) <= sideBySideColumnWidth {
		return value
	}
	if sideBySideColumnWidth < 4 {
		return value[:sideBySideColumnWidth]
	}
	return value[:sideBySideColumnWidth-3] + "..."
}

func formatSideBySideLine(left, right string) string {
	return fmt.Sprintf("%-*s | %-*s\n", sideBySideColumnWidth, truncateForColumn(left), sideBySideColumnWidth, truncateForColumn(right))
}

func formatDelta(delta int64) string {
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
}

// FormatSideBySide formats the diff as a side-by-side comparison.
func (d *Diff) FormatSideBySide() string {
	var b strings.Builder

	b.WriteString("=== Comparison Summary ===\n")
	b.WriteString(d.Summary + "\n")
	b.WriteString("\n")

	if d.StatusChanged || d.ErrorChanged {
		b.WriteString("=== Status/Error ===\n")
		b.WriteString(formatSideBySideLine("On-Chain", "Local"))
		b.WriteString(strings.Repeat("-", sideBySideColumnWidth*2+3) + "\n")
		b.WriteString(formatSideBySideLine("status: "+d.OnChainStatus, "status: "+d.LocalStatus))
		b.WriteString(formatSideBySideLine("error: "+d.OnChainError, "error: "+d.LocalError))
		b.WriteString("\n")
	}

	if d.BudgetDiff.Available {
		b.WriteString("=== Budget Usage ===\n")
		b.WriteString(fmt.Sprintf("cpu_instructions: on_chain=%d local=%d delta=%s\n", d.BudgetDiff.OnChain.CPUInstructions, d.BudgetDiff.Local.CPUInstructions, formatDelta(d.BudgetDiff.CPUOps)))
		b.WriteString(fmt.Sprintf("memory_bytes: on_chain=%d local=%d delta=%s\n", d.BudgetDiff.OnChain.MemoryBytes, d.BudgetDiff.Local.MemoryBytes, formatDelta(d.BudgetDiff.Memory)))
		b.WriteString(fmt.Sprintf("operations_count: on_chain=%d local=%d delta=%s\n", d.BudgetDiff.OnChain.OperationsCount, d.BudgetDiff.Local.OperationsCount, formatDelta(d.BudgetDiff.OpsCount)))
		b.WriteString("\n")
	}

	if countCallPathChanges(d.CallPathDiff) > 0 {
		b.WriteString("=== Divergent Call Paths ===\n")
		b.WriteString(formatSideBySideLine("On-Chain", "Local"))
		b.WriteString(strings.Repeat("-", sideBySideColumnWidth*2+3) + "\n")
		for _, cp := range d.CallPathDiff {
			if cp.Type == "unchanged" {
				continue
			}
			left := fmt.Sprintf("[%d][%s] %s", cp.Index, cp.Type, cp.OnChain)
			right := fmt.Sprintf("[%d][%s] %s", cp.Index, cp.Type, cp.Local)
			b.WriteString(formatSideBySideLine(left, right))
		}
		b.WriteString("\n")
	}

	if len(d.EventsDiff) > 0 {
		b.WriteString("=== Event Diff (On-Chain | Local) ===\n")
		b.WriteString(formatSideBySideLine("On-Chain", "Local"))
		b.WriteString(strings.Repeat("-", sideBySideColumnWidth*2+3) + "\n")
		changes := 0
		for _, ed := range d.EventsDiff {
			if ed.Type == "unchanged" {
				continue
			}
			changes++
			left := fmt.Sprintf("[%d][%s] %s", ed.Index, ed.Type, ed.OnChain)
			right := fmt.Sprintf("[%d][%s] %s", ed.Index, ed.Type, ed.Local)
			b.WriteString(formatSideBySideLine(left, right))
		}
		if changes == 0 {
			b.WriteString("No event differences.\n")
		}
		b.WriteString("\n")
	}

	if len(d.LogsDiff) > 0 {
		b.WriteString("=== Log Diff (On-Chain | Local) ===\n")
		b.WriteString(formatSideBySideLine("On-Chain", "Local"))
		b.WriteString(strings.Repeat("-", sideBySideColumnWidth*2+3) + "\n")
		changes := 0
		for _, ld := range d.LogsDiff {
			if ld.Type == "unchanged" {
				continue
			}
			changes++
			left := fmt.Sprintf("[%d][%s] %s", ld.Index, ld.Type, ld.OnChain)
			right := fmt.Sprintf("[%d][%s] %s", ld.Index, ld.Type, ld.Local)
			b.WriteString(formatSideBySideLine(left, right))
		}
		if changes == 0 {
			b.WriteString("No log differences.\n")
		}
	}

	return b.String()
}
