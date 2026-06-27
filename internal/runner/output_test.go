package runner

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputWriterPlain(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(false)
	entries := map[string]HostEntry{
		"www.example.com": {Host: "www.example.com", Source: "crtsh"},
		"api.example.com": {Host: "api.example.com", Source: "alienvault"},
	}
	err := ow.Write("example.com", entries, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "www.example.com") {
		t.Error("missing www.example.com")
	}
	if !strings.Contains(output, "api.example.com") {
		t.Error("missing api.example.com")
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestOutputWriterJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(true)
	entries := map[string]HostEntry{
		"www.example.com": {Host: "www.example.com", Source: "crtsh", Domain: "example.com"},
	}
	err := ow.Write("example.com", entries, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"host":"www.example.com"`) {
		t.Errorf("expected JSON host field, got: %s", output)
	}
	if !strings.Contains(output, `"source":"crtsh"`) {
		t.Errorf("expected JSON source field, got: %s", output)
	}
	if !strings.Contains(output, `"input":"example.com"`) {
		t.Errorf("expected JSON input field, got: %s", output)
	}
}

func TestOutputWriterHostIP(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(false)
	entries := map[string]HostEntry{
		"www.example.com": {Host: "www.example.com", Source: "crtsh", IPs: []string{"1.2.3.4", "5.6.7.8"}},
	}
	err := ow.WriteHostIP("example.com", entries, &buf)
	if err != nil {
		t.Fatalf("WriteHostIP failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "www.example.com,1.2.3.4,crtsh") {
		t.Errorf("expected host,ip,source format, got: %s", output)
	}
	if !strings.Contains(output, "www.example.com,5.6.7.8,crtsh") {
		t.Errorf("expected both IPs, got: %s", output)
	}
}

func TestOutputWriterHostIPNoIP(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(false)
	entries := map[string]HostEntry{
		"www.example.com": {Host: "www.example.com", Source: "crtsh"},
	}
	err := ow.WriteHostIP("example.com", entries, &buf)
	if err != nil {
		t.Fatalf("WriteHostIP failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "www.example.com,,crtsh") {
		t.Errorf("expected empty IP field, got: %s", output)
	}
}

func TestOutputWriterSourcesPlain(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(false)
	sourceMap := map[string]map[string]struct{}{
		"www.example.com": {"crtsh": {}, "alienvault": {}},
	}
	err := ow.WriteSources("example.com", sourceMap, &buf)
	if err != nil {
		t.Fatalf("WriteSources failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "www.example.com,[") {
		t.Errorf("expected source list format, got: %s", output)
	}
	if !strings.Contains(output, "crtsh") || !strings.Contains(output, "alienvault") {
		t.Errorf("expected both sources, got: %s", output)
	}
}

func TestOutputWriterSourcesJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(true)
	sourceMap := map[string]map[string]struct{}{
		"www.example.com": {"crtsh": {}},
	}
	err := ow.WriteSources("example.com", sourceMap, &buf)
	if err != nil {
		t.Fatalf("WriteSources failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"host":"www.example.com"`) {
		t.Errorf("expected JSON host, got: %s", output)
	}
	if !strings.Contains(output, `"sources":["crtsh"]`) {
		t.Errorf("expected JSON sources, got: %s", output)
	}
}

func TestOutputWriterEmptyEntries(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(false)
	err := ow.Write("example.com", map[string]HostEntry{}, &buf)
	if err != nil {
		t.Fatalf("Write with empty entries failed: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got: %s", buf.String())
	}
}

func TestOutputWriterCreateFile(t *testing.T) {
	t.Parallel()
	ow := NewOutputWriter(false)
	f, err := ow.createFile("", false)
	if err == nil {
		t.Error("expected error for empty filename")
		if f != nil {
			f.Close()
		}
	}
}

func TestOutputWriterJSONEmptySources(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(true)
	sourceMap := map[string]map[string]struct{}{}
	err := ow.WriteSources("example.com", sourceMap, &buf)
	if err != nil {
		t.Fatalf("WriteSources with empty map failed: %v", err)
	}
}

func TestOutputWriterSortOrder(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ow := NewOutputWriter(false)
	entries := map[string]HostEntry{
		"z.example.com": {Host: "z.example.com"},
		"a.example.com": {Host: "a.example.com"},
		"m.example.com": {Host: "m.example.com"},
	}
	err := ow.Write("example.com", entries, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a.example.com" {
		t.Errorf("expected a.example.com first, got %s", lines[0])
	}
	if lines[1] != "m.example.com" {
		t.Errorf("expected m.example.com second, got %s", lines[1])
	}
	if lines[2] != "z.example.com" {
		t.Errorf("expected z.example.com third, got %s", lines[2])
	}
}
