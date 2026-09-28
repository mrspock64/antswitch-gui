package main

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestLoadConfigMigratesLegacyFlatShape(t *testing.T) {
	dir := withTempHome(t)
	legacy := `{"host":"10.10.0.123","token":"banangroda","mini":true,"light":false}`
	if err := os.WriteFile(filepath.Join(dir, ".antswitch-gui.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig()

	if cfg.AT14.Host != "10.10.0.123" {
		t.Errorf("AT14.Host = %q, want 10.10.0.123", cfg.AT14.Host)
	}
	if cfg.AT14.Token != "banangroda" {
		t.Errorf("AT14.Token = %q, want banangroda", cfg.AT14.Token)
	}
	if !cfg.Mini {
		t.Error("Mini = false, want true")
	}
	if cfg.Device != DeviceAT14 {
		t.Errorf("Device = %q, want %q (legacy configs never had AS-1289)", cfg.Device, DeviceAT14)
	}
	if cfg.AS1289.Host != as1289DefaultHost {
		t.Errorf("AS1289.Host = %q, want default %q", cfg.AS1289.Host, as1289DefaultHost)
	}
	if len(cfg.AS1289.Names) != as1289PortCount {
		t.Errorf("len(AS1289.Names) = %d, want %d", len(cfg.AS1289.Names), as1289PortCount)
	}
}

func TestLoadConfigRoundTripsNewShape(t *testing.T) {
	withTempHome(t)

	want := Config{
		Device: DeviceAS1289,
		AT14:   AT14Config{Host: "antennswitch.local", Token: "tok"},
		AS1289: AS1289Config{
			Host:     "192.168.86.40",
			AuthUser: "admin",
			AuthPass: "secret",
			Names:    []string{"OCD", "EFHW", "", "", ""},
		},
		Mini:  true,
		Light: true,
	}
	if err := saveConfig(want); err != nil {
		t.Fatal(err)
	}

	got := loadConfig()
	if got.Device != want.Device {
		t.Errorf("Device = %q, want %q", got.Device, want.Device)
	}
	if got.AS1289.Host != want.AS1289.Host || got.AS1289.AuthUser != want.AS1289.AuthUser {
		t.Errorf("AS1289 = %+v, want %+v", got.AS1289, want.AS1289)
	}
	if len(got.AS1289.Names) != as1289PortCount || got.AS1289.Names[0] != "OCD" || got.AS1289.Names[1] != "EFHW" {
		t.Errorf("AS1289.Names = %v, want first two OCD/EFHW padded to %d", got.AS1289.Names, as1289PortCount)
	}
}

func TestLoadConfigDefaultsOnMissingFile(t *testing.T) {
	withTempHome(t)

	cfg := loadConfig()
	if cfg.Device != DeviceAT14 {
		t.Errorf("Device = %q, want %q", cfg.Device, DeviceAT14)
	}
	if cfg.AS1289.Host != as1289DefaultHost {
		t.Errorf("AS1289.Host = %q, want default %q", cfg.AS1289.Host, as1289DefaultHost)
	}
	if len(cfg.AS1289.Names) != as1289PortCount {
		t.Errorf("len(AS1289.Names) = %d, want %d", len(cfg.AS1289.Names), as1289PortCount)
	}
}
