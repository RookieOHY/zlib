package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/heartleo/zlib/internal/config"
	"github.com/heartleo/zlib/internal/zlib"
)

// openBoox handles /boox, /boox send [file] and /boox config in the shell.
func (m *shellModel) openBoox(arg string) tea.Cmd {
	if arg == "config" {
		cfg, err := config.LoadBooxConfig()
		if err != nil && !os.IsNotExist(err) {
			m.setError(err.Error())
			return nil
		}
		form := newInlineForm("Boox settings", "Save", []formField{
			{label: "Device IP", value: cfg.IP, hint: "shown by BooxDrop on the device, e.g. 192.168.1.23"},
			{label: "Import to library", value: strconv.FormatBool(cfg.Library), hint: "true imports into the library; false saves to the Books folder"},
		})
		form.validate = func(fields []formField) error {
			_, err := shellBooxConfig(fields)
			return err
		}
		return m.open("boox-config", form)
	}
	if path, ok := strings.CutPrefix(arg, "send "); ok {
		path = strings.TrimSpace(path)
		if strings.HasPrefix(path, "\"") {
			if !strings.HasSuffix(path, "\"") || len(path) < 2 {
				m.setError("Close the quoted file path")
				return nil
			}
			path = path[1 : len(path)-1]
		}
		if path == "" {
			m.setError("Use /boox send <file>")
			return nil
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			m.setError("File not found: " + path)
			return nil
		}
		m.booxPath = path
		return m.open("boox-confirm", shellConfirm{title: "Send to Boox?", detail: filepath.Base(path), action: "Send"})
	}
	if arg != "" && arg != "send" {
		m.setError("Use /boox send <file> or /boox config")
		return nil
	}
	dir := filePickerStartDir()
	return m.open("boox-file", newBooxFileModel(dir))
}

func shellBooxConfig(fields []formField) (zlib.BooxConfig, error) {
	ip := strings.TrimSpace(fields[0].value)
	if ip == "" {
		return zlib.BooxConfig{}, fmt.Errorf("device IP is required; BooxDrop shows it on the device")
	}
	library := parseBoolLoose(fields[1].value)
	return zlib.BooxConfig{IP: ip, Library: library}, nil
}

func parseBoolLoose(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "y", "1", "on":
		return true
	}
	return false
}

func (m *shellModel) saveBoox(fields []formField) tea.Cmd {
	cfg, err := shellBooxConfig(fields)
	if err == nil {
		err = config.SaveBooxConfig(cfg)
	}
	if err != nil {
		if form, ok := m.page.(inlineForm); ok {
			form.accepted = false
			form.message = "Could not save Boox settings: " + err.Error()
			m.page = form
		}
		return nil
	}
	mode := "the Books folder"
	if cfg.Library {
		mode = "the device library"
	}
	m.openInfo("Boox settings saved", "Device\n"+bookCell(cfg.IP)+"\n\nUploads land in\n"+bookCell(mode))
	return nil
}

func (m *shellModel) sendBoox(path string) tea.Cmd {
	cfg, err := config.LoadBooxConfig()
	if err != nil || cfg.IP == "" {
		if m.deliveryReturn != nil {
			return m.returnToDownloadCompletion(noticeInfo, "Set up Boox with /boox config")
		}
		m.openInfo("Boox setup required", "Save your device IP with /boox config\n(The IP is shown by BooxDrop on the device.)")
		return nil
	}
	return m.open("boox-send", newBooxSendModel(path, cfg))
}

// — boox send model (spinner during BooxDrop upload) —

type booxSendModel struct {
	width, height int
	filePath      string
	cfg           zlib.BooxConfig
	spinner       spinner.Model
	err           error
	done          bool
}

type booxSendDoneMsg struct {
	dest string
}

func newBooxSendModel(filePath string, cfg zlib.BooxConfig) booxSendModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	return booxSendModel{filePath: filePath, cfg: cfg, spinner: s}
}

func (m booxSendModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.sendCmd())
}

func (m booxSendModel) sendCmd() tea.Cmd {
	return func() tea.Msg {
		dest, err := zlib.SendToBoox(m.filePath, m.cfg)
		if err != nil {
			return errMsg{err}
		}
		return booxSendDoneMsg{dest: dest}
	}
}

func (m booxSendModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	case booxSendDoneMsg:
		m.done = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("stopped waiting; the upload may still complete")
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m booxSendModel) View() tea.View {
	return screenView(m.width, m.height, "Send to Boox", bookCell(m.filePath), fmt.Sprintf("  %s Uploading to %s...\n", m.spinner.View(), m.cfg.IP), "", uiKey("ctrl+c", "stop waiting"))
}
