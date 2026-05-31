package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintNoticeUsesHefUpdateCommand(t *testing.T) {
	var out bytes.Buffer
	previous := updateNoticeWriter
	updateNoticeWriter = &out
	defer func() { updateNoticeWriter = previous }()

	printNotice("v1.0.0", "v1.1.0")

	output := out.String()
	for _, want := range []string{"Update available: v1.0.0 -> v1.1.0", `Run "hef update" to upgrade.`} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}
