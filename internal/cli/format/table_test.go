package format_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tkr41850-debug/mcp-chain/internal/cli/format"
	"github.com/tkr41850-debug/mcp-chain/internal/store"
)

func TestWriteTable_EmptyIn_EmptyOut(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, nil))
	require.Empty(t, buf.String())

	require.NoError(t, format.WriteTable(&buf, []store.Record{}))
	require.Empty(t, buf.String())
}

func TestWriteTable_NilResolvedAtRendersDash(t *testing.T) {
	rec := store.Record{
		ID:         "acid",
		Status:     "pending",
		Condition:  "wait for build",
		CreatedAt:  time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
		ResolvedAt: nil,
	}
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, []store.Record{rec}))
	out := buf.String()
	require.Contains(t, out, "acid")
	require.Contains(t, out, "pending")
	require.Contains(t, out, "2026-04-24T10:00:00Z")

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2, "header + 1 row")
	require.True(t, strings.HasSuffix(strings.TrimRight(lines[1], " "), "-"),
		"resolved-nil row ends with '-' (after trimming tabwriter right-padding)")
}

func TestWriteTable_TruncatesLongConditionWithEllipsis(t *testing.T) {
	long := strings.Repeat("x", 100)
	rec := store.Record{
		ID:        "a",
		Status:    "pending",
		Condition: long,
		CreatedAt: time.Unix(0, 0).UTC(),
	}
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, []store.Record{rec}))
	out := buf.String()
	// 48-cap with "..." suffix → 45 x's + "...".
	require.Contains(t, out, strings.Repeat("x", 45)+"...")
	require.NotContains(t, out, strings.Repeat("x", 49),
		"truncation must cut at 45 x's (48 - len('...'))")
}

func TestWriteTable_SortsByCreatedAtThenID(t *testing.T) {
	t0 := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	recs := []store.Record{
		{ID: "zebra", Status: "pending", Condition: "c", CreatedAt: t0.Add(2 * time.Second)},
		{ID: "acid", Status: "pending", Condition: "c", CreatedAt: t0.Add(1 * time.Second)},
		{ID: "b-tied", Status: "pending", Condition: "c", CreatedAt: t0.Add(1 * time.Second)},
	}
	var buf bytes.Buffer
	require.NoError(t, format.WriteTable(&buf, recs))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 4, "header + 3 rows")
	require.Contains(t, lines[1], "acid", "acid at t+1 comes first")
	require.Contains(t, lines[2], "b-tied", "b-tied at t+1 sorts by ID after acid")
	require.Contains(t, lines[3], "zebra", "zebra at t+2 last")
}
