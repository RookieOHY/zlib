package cli

import (
	"strings"
	"testing"

	"github.com/heartleo/zlib/internal/config"
	zlib "github.com/heartleo/zlib/internal/zlib"
)

func TestShellBooxCommandListed(t *testing.T) {
	found := false
	for _, c := range shellCommands {
		if c.name == "boox" {
			found = true
		}
	}
	if !found {
		t.Fatal("/boox missing from the command list")
	}
}

func TestShellBooxConfigFormRequiresIP(t *testing.T) {
	isolateConfigHome(t)
	m := newShellModel()
	m.dispatch("/boox config")
	form, ok := m.page.(inlineForm)
	if !ok || m.kind != "boox-config" {
		t.Fatalf("kind = %q, want the Boox settings form", m.kind)
	}
	if err := form.validate(form.fields); err == nil || !strings.Contains(err.Error(), "IP") {
		t.Fatalf("empty device IP accepted: %v", err)
	}
}

func TestShellBooxConfigParsesLibraryMode(t *testing.T) {
	cfg, err := shellBooxConfig([]formField{{value: "192.168.1.23"}, {value: "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IP != "192.168.1.23" || !cfg.Library {
		t.Fatalf("cfg = %+v, want IP 192.168.1.23 with library import", cfg)
	}
}

func TestShellBooxSendRequiresSetup(t *testing.T) {
	isolateConfigHome(t)
	m := newShellModel()
	path := writeTempFile(t, "book.epub")
	m.sendBoox(path)
	if m.infoTitle != "Boox setup required" {
		t.Fatalf("infoTitle = %q, want the setup notice", m.infoTitle)
	}
}

func TestShellBooxSendOpensUploadPage(t *testing.T) {
	isolateConfigHome(t)
	if err := config.SaveBooxConfig(zlib.BooxConfig{IP: "192.168.1.23"}); err != nil {
		t.Fatal(err)
	}
	m := newShellModel()
	path := writeTempFile(t, "book.epub")
	m.sendBoox(path)
	if m.kind != "boox-send" {
		t.Fatalf("kind = %q, want the Boox upload page", m.kind)
	}
	if _, ok := m.page.(booxSendModel); !ok {
		t.Fatal("boox-send page is not a booxSendModel")
	}
}

func TestShellBooxSendFileConfirmFlow(t *testing.T) {
	isolateConfigHome(t)
	if err := config.SaveBooxConfig(zlib.BooxConfig{IP: "192.168.1.23"}); err != nil {
		t.Fatal(err)
	}
	m := newShellModel()
	path := writeTempFile(t, "book.epub")
	m.dispatch("/boox send " + path)
	if m.kind != "boox-confirm" {
		t.Fatalf("kind = %q, want the Boox confirm page", m.kind)
	}
}
