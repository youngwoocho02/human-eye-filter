package sieve

import (
	"strings"
	"testing"
)

func TestReduceGroupsGrepOutput(t *testing.T) {
	input := strings.Join([]string{
		"Assets/Foo.cs:10: first",
		"Assets/Foo.cs:20: second",
		"Assets/Bar.cs:5: other",
	}, "\n")

	output, err := Reduce(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"Assets/Foo.cs (2)", "10: first", "Assets/Bar.cs (1)", "mode=grep"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestReduceSamplesJSON(t *testing.T) {
	output, err := Reduce(`{"items":[1,2,3,4,5,6,7,8,9,10],"message":"ok"}`, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{`"_sieve_omitted_items":2`, `"message":"ok"`, "mode=json"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestReduceCollapsesDuplicateText(t *testing.T) {
	output, err := Reduce("error: boom\nerror: boom\nerror: boom\nnext\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "<repeated 3 times>") {
		t.Fatalf("expected duplicate collapse:\n%s", output)
	}
}

func TestReduceGroupsPaths(t *testing.T) {
	options := DefaultOptions()
	options.Mode = "paths"
	output, err := Reduce("Assets/A/Foo.cs\nAssets/A/Bar.cs\nAssets/B/Baz.asset\n", options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"Assets/A/ (2)", "Foo.cs", "Assets/B/ (1)"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestToolHintDoesNotOverrideJSON(t *testing.T) {
	options := DefaultOptions()
	options.Tool = "gh"
	output, err := Reduce(`{"items":[1,2,3,4,5,6,7,8,9,10],"message":"ok"}`, options)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "mode=json") {
		t.Fatalf("expected json mode despite tool hint:\n%s", output)
	}
}

func TestSCMChangedLinesArePriority(t *testing.T) {
	options := DefaultOptions()
	options.Tool = "cm"
	options.MaxLines = 4
	output, err := Reduce("noise\nCH Assets/Foo.cs\nCO+CH Assets/Bar.cs\n", options)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "CH Assets/Foo.cs") || !strings.Contains(output, "CO+CH Assets/Bar.cs") {
		t.Fatalf("expected SCM changed lines:\n%s", output)
	}
}

func TestPathReducerSkipsNonPathLines(t *testing.T) {
	options := DefaultOptions()
	options.Mode = "paths"
	output, err := Reduce("total 10\n-rw-r--r-- 1 user file\nAssets/A/Foo.cs\n", options)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output, "total 10") || strings.Contains(output, "-rw-r--r--") {
		t.Fatalf("expected non-path lines skipped:\n%s", output)
	}
	if !strings.Contains(output, "Foo.cs") {
		t.Fatalf("expected path line retained:\n%s", output)
	}
}

func TestUTF8TruncationKeepsValidText(t *testing.T) {
	options := DefaultOptions()
	options.MaxChars = 8
	output, err := Reduce("에러에러에러에러\n", options)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output, "\ufffd") {
		t.Fatalf("invalid UTF-8 replacement appeared:\n%s", output)
	}
}

func TestUnityReducerKeepsStackFramesAfterException(t *testing.T) {
	options := DefaultOptions()
	options.Mode = "unity"
	output, err := Reduce(strings.Join([]string{
		"noise before",
		"System.Exception: boom",
		"  at Foo.Bar()",
		"  at Foo.Baz()",
		"noise after",
	}, "\n"), options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"System.Exception: boom", "at Foo.Bar()", "at Foo.Baz()"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected stack frame %q in output:\n%s", want, output)
		}
	}
}
