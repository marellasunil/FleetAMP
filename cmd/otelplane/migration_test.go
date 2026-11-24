package main

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInspectImportedConfigurationInventoriesCollectorComponents(t *testing.T) {
	components, warnings, err := inspectImportedConfiguration(`
receivers:
  prometheus: {}
  otlp: {}
processors:
  batch: {}
exporters:
  otlphttp: {}
service:
  pipelines:
    metrics:
      receivers: [prometheus]
    traces:
      receivers: [otlp]
custom_section: {}
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(components[0].Names, ","); got != "otlp,prometheus" {
		t.Fatalf("receivers=%q", got)
	}
	if got := strings.Join(components[len(components)-1].Names, ","); got != "metrics,traces" {
		t.Fatalf("pipelines=%q", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "custom_section") {
		t.Fatalf("warnings=%v", warnings)
	}
}

func TestInspectImportedConfigurationRejectsInvalidStructures(t *testing.T) {
	for name, content := range map[string]string{
		"invalid yaml":       "receivers: [",
		"non-mapping section": "receivers: []",
		"empty document":      "{}",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := inspectImportedConfiguration(content)
			if err == nil {
				t.Fatal("expected import to fail")
			}
		})
	}
}

func TestReadMigrationUploadAcceptsYAMLAndRejectsOtherExtensions(t *testing.T) {
	for _, test := range []struct {
		name      string
		fileName  string
		wantError bool
	}{
		{name: "yaml", fileName: "collector.yaml"},
		{name: "yml", fileName: "collector.yml"},
		{name: "text", fileName: "collector.txt", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("yaml_file", test.fileName)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write([]byte("receivers: {}")); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("POST", "/migration", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			if err := request.ParseMultipartForm(migrationUploadLimit); err != nil {
				t.Fatal(err)
			}
			content, fileName, err := readMigrationUpload(request)
			if test.wantError {
				if err == nil {
					t.Fatal("expected extension error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if content != "receivers: {}" || fileName != test.fileName {
				t.Fatalf("content=%q filename=%q", content, fileName)
			}
		})
	}
}
