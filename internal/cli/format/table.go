// Package format renders store.Record slices into operator-friendly
// output. Currently supports WriteTable (aligned 5-column ASCII table
// via stdlib text/tabwriter). Lives in its own sub-package so
// presentation concerns never leak into internal/store.
//
// WriteTable is PURE: no store access, no env-var reads, no os.Stdout
// fallback. Empty input yields zero output (caller owns the
// "no entries" hint). See LD-11 in 07-01-PLAN.md.
package format

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/anthropics/mcp-chain/internal/store"
)

// conditionMaxWidth caps the CONDITION column so a pathological
// multi-line condition can't wreck alignment. 48 is the CONTEXT.md
// lock (LD-9). ASCII assumption is fine for a cap this generous.
const conditionMaxWidth = 48

// tsFormat is RFC3339 in UTC (LD-10). Fixed-width (20 chars w/ 'Z' suffix).
const tsFormat = "2006-01-02T15:04:05Z07:00"

// WriteTable renders records as an aligned 5-column table to w.
// Column order (LOCKED): ID, STATUS, CONDITION, CREATED, RESOLVED.
// Sort order (LOCKED): CreatedAt ASC, ties broken by ID ASC.
// Empty input produces NO output (LD-11: caller owns stderr hint).
// Returns any Flush error.
func WriteTable(w io.Writer, records []store.Record) error {
	if len(records) == 0 {
		return nil
	}
	sorted := make([]store.Record, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].ID < sorted[j].ID
	})

	// minwidth=0, tabwidth=0, padding=2, padchar=' ', flags=0
	// → two-space minimum separator, left-aligned, no ANSI.
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// tabwriter buffers writes; errors surface on Flush() below.
	_, _ = fmt.Fprintln(tw, "ID\tSTATUS\tCONDITION\tCREATED\tRESOLVED")
	for _, r := range sorted {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			r.ID,
			r.Status,
			truncate(r.Condition, conditionMaxWidth),
			r.CreatedAt.UTC().Format(tsFormat),
			formatResolvedAt(r.ResolvedAt),
		)
	}
	return tw.Flush() // Pitfall 8: MUST flush or trailing rows vanish.
}

// truncate returns s unchanged if len(s) <= max, else s[:max-3] + "...".
// The length guard (LD-9 / Pitfall 7) prevents a panic on short strings.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// formatResolvedAt returns the RFC3339 UTC timestamp, or "-" for nil.
// Tabwriter right-pads either length to the column width (LD-10).
func formatResolvedAt(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format(tsFormat)
}
