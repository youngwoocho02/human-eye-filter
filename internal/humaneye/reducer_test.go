package humaneye

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

	for _, want := range []string{`"_hef_omitted_items":2`, `"message":"ok"`, "mode=json"} {
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

func TestSCMReducerSplitsCheckoutOnlyFromContentChanges(t *testing.T) {
	options := DefaultOptions()
	options.Tool = "cm"
	output, err := Reduce("CO Assets/Locked.asset\nCO+CH Assets/Changed.asset\nCH Assets/Edited.cs\n", options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"## content change", "CO+CH Assets/Changed.asset", "CH Assets/Edited.cs", "## checkout only", "CO Assets/Locked.asset"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestFocusKeywordIsKeptInLogReducer(t *testing.T) {
	options := DefaultOptions()
	options.Mode = "unity"
	options.Focus = "DemoScope"
	output, err := Reduce("noise\nDemoScope blocked DLC_01\nanother noise\n", options)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "DemoScope blocked DLC_01") {
		t.Fatalf("expected focus line:\n%s", output)
	}
}

func TestSCMFocusDoesNotOverrideConflictOrChangeSections(t *testing.T) {
	options := DefaultOptions()
	options.Tool = "cm"
	options.Focus = "DemoScope"
	output, err := Reduce("CONFLICT DemoScope.asset\nCO+CH Assets/DemoScope.asset\nplain DemoScope note\n", options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"## conflicts", "CONFLICT DemoScope.asset", "## content change", "CO+CH Assets/DemoScope.asset", "## focused", "plain DemoScope note"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestGrepReducerAliasesCommonRoot(t *testing.T) {
	input := strings.Join([]string{
		"C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/A/Foo.cs:10: first",
		"C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/B/Bar.cs:20: second",
	}, "\n")

	output, err := Reduce(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"$root=C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts", "$root/A/Foo.cs", "$root/B/Bar.cs"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestGrepReducerKeepsRipgrepColumn(t *testing.T) {
	output, err := Reduce("Assets/Foo.cs:10:24: matched text\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "10:24: matched text") {
		t.Fatalf("expected line and column location:\n%s", output)
	}
}

func TestReducerStripsANSIEscapeSequencesBeforeParsing(t *testing.T) {
	output, err := Reduce("\x1b[31mAssets/Foo.cs:10: error: boom\x1b[0m\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"Assets/Foo.cs (1)", "10: error: boom"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("expected ANSI escapes to be stripped:\n%s", output)
	}
}

func TestReducerDropsNULBytesBeforeParsing(t *testing.T) {
	output, err := Reduce("Assets/Foo.cs:10:\x00 error after nul\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(output, "\x00") || !strings.Contains(output, "error after nul") {
		t.Fatalf("expected NUL byte to be removed:\n%s", output)
	}
}

func TestPathReducerAliasesCommonRoot(t *testing.T) {
	options := DefaultOptions()
	options.Mode = "paths"
	input := strings.Join([]string{
		"C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/A/Foo.cs",
		"C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/B/Bar.cs",
	}, "\n")

	output, err := Reduce(input, options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"$root=C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts", "$root/A/", "$root/B/"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
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

func TestOutputLimitReportsOmittedLines(t *testing.T) {
	options := DefaultOptions()
	options.MaxLines = 2
	output, err := Reduce("one\ntwo\nthree\nfour\n", options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"output limit reached", "omitted 2 more reduced lines", "Narrow the command"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}

func TestOutputLimitReportsOmittedChars(t *testing.T) {
	options := DefaultOptions()
	options.MaxChars = 5
	output, err := Reduce("abcdefghi\n", options)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"output limit reached", "omitted", "Narrow the command"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}
