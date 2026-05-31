package humaneye

import (
	"strconv"
	"strings"
	"testing"
)

func TestRepeatedLineFilter(t *testing.T) {
	output, err := Reduce("loading\nloading\nloading\nerror: failed\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "loading (x3)")
	assertContains(t, output, "filters=RepeatedLine")
}

func TestSequentialRangeFilter(t *testing.T) {
	output, err := Reduce("file_001.tmp\nfile_002.tmp\nfile_003.tmp\nfile_004.tmp\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "file_001..004.tmp (4 lines)")
}

func TestSequentialRangeDoesNotCrossOtherLines(t *testing.T) {
	output, err := Reduce("file_001.tmp\nother\nfile_002.tmp\nfile_003.tmp\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "file_001..003") {
		t.Fatalf("range crossed unrelated line:\n%s", output)
	}
}

func TestPathLineGroupFilter(t *testing.T) {
	lines := []string{}
	for i := 1; i <= 12; i++ {
		lines = append(lines, "C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/Domain/Foo/FooController.cs:"+strconv.Itoa(i)+": repeated match")
	}
	for i := 1; i <= 6; i++ {
		lines = append(lines, "C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/Domain/Bar/BarPresenter.cs:"+strconv.Itoa(i)+": repeated match")
	}
	output, err := Reduce(strings.Join(lines, "\n"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "FooController.cs (12)")
	assertContains(t, output, "... <8 more>")
	assertContains(t, output, "BarPresenter.cs (6)")
	assertContains(t, output, "filters=PathLineGroup")
}

func TestDirectoryGroupFilter(t *testing.T) {
	lines := []string{}
	for i := 0; i < 12; i++ {
		lines = append(lines, "C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/Domain/CookStation/A/File"+strconv.Itoa(i)+".cs")
	}
	for i := 0; i < 10; i++ {
		lines = append(lines, "C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/Domain/CookStation/B/File"+strconv.Itoa(i)+".cs")
	}
	output, err := Reduce(strings.Join(lines, "\n"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "$root=C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts/Domain/CookStation")
	assertContains(t, output, "$root/A/ (12)")
	assertContains(t, output, "... <4 more>")
	assertContains(t, output, "filters=DirectoryGroup")
}
func TestCommonPrefixFilter(t *testing.T) {
	output, err := Reduce("Namespace.Project.Feature.Alpha\nNamespace.Project.Feature.Beta\nNamespace.Project.Feature.Gamma\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "$prefix1=Namespace.Project.Feature.")
	assertContains(t, output, "$prefix1Alpha")
}

func TestDictionaryTokenFilter(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	output, err := Reduce("created "+id+"\nupdated "+id+"\ndeleted "+id+"\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "$t1="+id)
	assertContains(t, output, "created $t1")
}

func TestBoundedSampleFilter(t *testing.T) {
	options := DefaultOptions()
	options.MaxLines = 12
	lines := []string{}
	for i := 0; i < 40; i++ {
		lines = append(lines, "line "+string(rune('a'+(i%20))))
	}
	lines[20] = "fatal: access denied"
	output, err := Reduce(strings.Join(lines, "\n"), options)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "## important")
	assertContains(t, output, "fatal: access denied")
	assertContains(t, output, "filters=BoundedSample")
}

func TestNormalizeRemovesAnsiAndNul(t *testing.T) {
	output, err := Reduce("\x1b[31merror\x1b[0m\x00\n", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "\x1b") || strings.Contains(output, "\x00") {
		t.Fatalf("expected control chars removed:\n%s", output)
	}
	assertContains(t, output, "error")
}

func assertContains(t *testing.T, value string, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("expected %q in:\n%s", want, value)
	}
}
