package app

import "testing"

func TestLastPanePreviewUsesLastResponseBlock(t *testing.T) {
	pane := `thinking

I updated the renderer so grouped rows now sit under the directory header with more spacing.
Tests pass locally.

~/Files/Fleet:main  /status`

	got := lastPanePreview(pane)
	want := "I updated the renderer so grouped rows now sit under the directory header with more spacing. Tests pass …"
	if got != want {
		t.Fatalf("lastPanePreview() = %q, want %q", got, want)
	}
}

func TestLastPanePreviewSummarizesLongBlock(t *testing.T) {
	pane := `Here is a longer response with enough detail that the dashboard should show only the first few words instead of flooding the row with everything.`

	got := lastPanePreview(pane)
	want := "Here is a longer response with enough detail that the dashboard should show only the first few words …"
	if got != want {
		t.Fatalf("lastPanePreview() = %q, want %q", got, want)
	}
}

func TestLastPanePreviewFallsBackWhenOnlyPrompt(t *testing.T) {
	pane := `~/Files/Fleet:main  /status`

	got := lastPanePreview(pane)
	if got != "waiting for output" {
		t.Fatalf("lastPanePreview() = %q, want waiting for output", got)
	}
}

func TestTerminalPanePreviewKeepsActualRecentTerminal(t *testing.T) {
	pane := "\x1b[31mold\x1b[0m\n\nassistant response\n~/Files/Fleet:main  /status\n"

	got := terminalPanePreview(pane)
	want := "old\n\nassistant response\n~/Files/Fleet:main  /status"
	if got != want {
		t.Fatalf("terminalPanePreview() = %q, want %q", got, want)
	}
}
