package cli

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/heartleo/zlib/internal/config"
	"github.com/heartleo/zlib/internal/zlib"
	"github.com/spf13/cobra"
)

var booxCmd = &cobra.Command{
	Use:   "boox",
	Short: "Send files to a Boox e-reader over the local network",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var booxFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find Boox devices on the local network",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		lipgloss.Fprintln(os.Stderr, "Scanning the local network for Boox devices...")
		devices, err := zlib.BooxScan()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			lipgloss.Fprintln(os.Stdout, fmt.Sprintf("%s No Boox device found. Open %s on the device and keep it on the same Wi-Fi.", colorFaint(symbolInfo), colorCyan("BooxDrop")))
			return nil
		}
		for _, dev := range devices {
			lipgloss.Fprintf(os.Stdout, "%s %s (%s)\n", colorGreen(symbolSuccess), colorCyan(dev.IP), dev.Model)
		}
		lipgloss.Fprintf(os.Stdout, "%s Save one as default: zlib boox config --ip <address>\n", colorFaint(symbolInfo))
		return nil
	},
}

var booxConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Set or show the default Boox device",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ip, _ := cmd.Flags().GetString("ip")
		library, _ := cmd.Flags().GetBool("library")
		if strings.TrimSpace(ip) != "" {
			cfg := zlib.BooxConfig{IP: strings.TrimSpace(ip), Library: library}
			if err := config.SaveBooxConfig(cfg); err != nil {
				return err
			}
			mode := "Books folder"
			if cfg.Library {
				mode = "device library"
			}
			lipgloss.Fprintf(os.Stdout, "%s Default Boox device saved: %s (uploads land in the %s)\n", colorGreen(symbolSuccess), colorCyan(cfg.IP), mode)
			return nil
		}
		cfg, err := config.LoadBooxConfig()
		if err != nil {
			lipgloss.Fprintln(os.Stdout, fmt.Sprintf("%s No default Boox device yet; run: %s", colorFaint(symbolInfo), colorCyan("zlib boox config --ip <address>")))
			return nil
		}
		mode := "Books folder"
		if cfg.Library {
			mode = "device library"
		}
		lipgloss.Fprintf(os.Stdout, "Default Boox device: %s (uploads land in the %s)\n", colorCyan(cfg.IP), mode)
		return nil
	},
}

var booxSendCmd = &cobra.Command{
	Use:   "send <file>",
	Short: "Send a local file to a Boox device",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("pass the file to send, e.g. zlib boox send ./book.epub")
		}
		filePath, err := expandTilde(args[0])
		if err != nil {
			return err
		}
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("file not found: %s", filePath)
		}
		cfg, err := resolveBooxTarget(cmd)
		if err != nil {
			return err
		}
		lipgloss.Fprintf(os.Stderr, "Sending to Boox at %s...\n", cfg.IP)
		dest, err := zlib.SendToBoox(filePath, cfg)
		if err != nil {
			return err
		}
		lipgloss.Fprintf(os.Stdout, "%s Sent %s to Boox at %s (%s)\n", colorGreen(symbolSuccess), colorCyan(filePath), colorCyan(cfg.IP), dest)
		return nil
	},
}

// resolveBooxTarget picks the device from --ip, the saved config, or a
// network scan; a single scan hit is remembered as the default.
func resolveBooxTarget(cmd *cobra.Command) (zlib.BooxConfig, error) {
	ip, _ := cmd.Flags().GetString("ip")
	library, _ := cmd.Flags().GetBool("library")
	if strings.TrimSpace(ip) != "" {
		return zlib.BooxConfig{IP: strings.TrimSpace(ip), Library: library}, nil
	}
	cfg, err := config.LoadBooxConfig()
	if err == nil && cfg.IP != "" {
		if library {
			cfg.Library = true
		}
		return cfg, nil
	} else if err != nil && !os.IsNotExist(err) {
		return zlib.BooxConfig{}, err
	}

	lipgloss.Fprintln(os.Stderr, "No saved Boox device; scanning the local network...")
	devices, err := zlib.BooxScan()
	if err != nil {
		return zlib.BooxConfig{}, err
	}
	if len(devices) == 0 {
		return zlib.BooxConfig{}, fmt.Errorf("no Boox device found; open %s on the device, then retry or set one with %s", colorCyan("BooxDrop"), colorCyan("zlib boox config --ip <address>"))
	}
	if len(devices) > 1 {
		var lines []string
		for _, dev := range devices {
			lines = append(lines, fmt.Sprintf("  %s (%s)", dev.IP, dev.Model))
		}
		return zlib.BooxConfig{}, fmt.Errorf("multiple Boox devices found:\n%s\npick one with --ip or %s", strings.Join(lines, "\n"), colorCyan("zlib boox config --ip <address>"))
	}
	cfg = zlib.BooxConfig{IP: devices[0].IP, Library: library}
	_ = config.SaveBooxConfig(cfg)
	lipgloss.Fprintf(os.Stderr, "%s Found %s (%s); saved as default\n", symbolInfo, colorCyan(cfg.IP), devices[0].Model)
	return cfg, nil
}

func init() {
	booxCmd.AddCommand(booxFindCmd, booxConfigCmd, booxSendCmd)
	booxSendCmd.Flags().String("ip", "", "Boox device IP (default: saved config or network scan)")
	booxSendCmd.Flags().Bool("library", false, "Import into the device library instead of the Books folder")
	booxConfigCmd.Flags().String("ip", "", "Boox device IP to save as default")
	booxConfigCmd.Flags().Bool("library", false, "Make library import the default upload mode")
}
