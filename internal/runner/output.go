package runner

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var errEmptyFilename = errors.New("empty filename")

type HostEntry struct {
	Host                string
	Domain              string
	Source              string
	IPs                 []string
	WildcardCertificate bool
}

type OutputWriter struct {
	JSON bool
}

func NewOutputWriter(json bool) *OutputWriter {
	return &OutputWriter{JSON: json}
}

func (o *OutputWriter) createFile(filename string, appendMode bool) (*os.File, error) {
	if filename == "" {
		return nil, errEmptyFilename
	}
	dir := filepath.Dir(filename)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	var file *os.File
	var err error
	if appendMode {
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	} else {
		file, err = os.Create(filename)
	}
	return file, err
}

func sortedKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})
	return keys
}

func (o *OutputWriter) Write(input string, entries map[string]HostEntry, writer io.Writer) error {
	if o.JSON {
		return writeJSON(input, entries, writer)
	}
	return writePlain(entries, writer)
}

func writePlain(entries map[string]HostEntry, writer io.Writer) error {
	bufw := bufio.NewWriter(writer)
	for _, host := range sortedKeys(entries) {
		if _, err := fmt.Fprintln(bufw, host); err != nil {
			return err
		}
	}
	return bufw.Flush()
}

func writeJSON(input string, entries map[string]HostEntry, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	for _, host := range sortedKeys(entries) {
		entry := entries[host]
		rec := jsonResult{
			Host:                host,
			Input:               input,
			Source:              entry.Source,
			WildcardCertificate: entry.WildcardCertificate,
		}
		if err := encoder.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

func (o *OutputWriter) WriteHostIP(_ string, entries map[string]HostEntry, writer io.Writer) error {
	bufw := bufio.NewWriter(writer)
	for _, host := range sortedKeys(entries) {
		entry := entries[host]
		for _, ip := range entry.IPs {
			line := fmt.Sprintf("%s,%s,%s\n", host, ip, entry.Source)
			if _, err := bufw.WriteString(line); err != nil {
				return err
			}
		}
		if len(entry.IPs) == 0 {
			line := fmt.Sprintf("%s,,%s\n", host, entry.Source)
			if _, err := bufw.WriteString(line); err != nil {
				return err
			}
		}
	}
	return bufw.Flush()
}

func (o *OutputWriter) WriteSources(input string, sourceMap map[string]map[string]struct{}, writer io.Writer) error {
	if o.JSON {
		return writeSourcesJSON(input, sourceMap, writer)
	}
	return writeSourcesPlain(sourceMap, writer)
}

func writeSourcesPlain(sourceMap map[string]map[string]struct{}, writer io.Writer) error {
	bufw := bufio.NewWriter(writer)
	for _, host := range sortedKeys(sourceMap) {
		sources := make([]string, 0, len(sourceMap[host]))
		for s := range sourceMap[host] {
			sources = append(sources, s)
		}
		sort.Strings(sources)
		line := fmt.Sprintf("%s,[%s]\n", host, strings.Join(sources, ","))
		if _, err := bufw.WriteString(line); err != nil {
			return err
		}
	}
	return bufw.Flush()
}

func writeSourcesJSON(input string, sourceMap map[string]map[string]struct{}, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	for _, host := range sortedKeys(sourceMap) {
		sources := make([]string, 0, len(sourceMap[host]))
		for s := range sourceMap[host] {
			sources = append(sources, s)
		}
		sort.Strings(sources)
		rec := jsonSourcesResult{
			Host:    host,
			Input:   input,
			Sources: sources,
		}
		if err := encoder.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

type jsonResult struct {
	Host                string `json:"host"`
	Input               string `json:"input"`
	Source              string `json:"source"`
	WildcardCertificate bool   `json:"wildcard_certificate,omitempty"`
}

type jsonSourcesResult struct {
	Host    string   `json:"host"`
	Input   string   `json:"input"`
	Sources []string `json:"sources"`
}
