package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Provider struct {
	AggregateDailyCapEnabled                                            bool
	State, Message                                                      string
	APIKey, Account, ImportMPAN, ImportSerial, ExportMPAN, ExportSerial string
	Token, PlantID, DeviceSerial                                        string
	RefreshInterval, LatestInterval, HistoryInterval                    time.Duration
}
type Config struct {
	DumpDir                               string
	BackupInterval, BackupTimeout         time.Duration
	HTTPAddr, DataDir, Username, Password string
	HistoryStart                          time.Time
	Timezone                              *time.Location
	ShutdownTimeout                       time.Duration
	Octopus, Growatt                      Provider
}

func FromEnv() (Config, error) { return Parse(os.Getenv, time.Now()) }
func Parse(get func(string) string, now time.Time) (Config, error) {
	value := func(k, d string) string {
		if v := get(k); v != "" {
			return v
		}
		return d
	}
	c := Config{HTTPAddr: value("ENERGYTHING_HTTP_ADDR", ":8080"), DataDir: value("ENERGYTHING_DATA_DIR", "./energything-runtime"), Username: get("ENERGYTHING_USERNAME"), Password: get("ENERGYTHING_PASSWORD")}
	c.DumpDir = value("ENERGYTHING_DUMP_DIR", "./energything-dumps")
	var backupErr error
	c.BackupInterval, backupErr = time.ParseDuration(value("ENERGYTHING_BACKUP_INTERVAL", "15m"))
	if backupErr != nil || c.BackupInterval <= 0 {
		return c, errors.New("ENERGYTHING_BACKUP_INTERVAL must be a positive duration")
	}
	c.BackupTimeout, backupErr = time.ParseDuration(value("ENERGYTHING_BACKUP_TIMEOUT", "5m"))
	if backupErr != nil || c.BackupTimeout <= 0 {
		return c, errors.New("ENERGYTHING_BACKUP_TIMEOUT must be a positive duration")
	}
	if c.Username == "" || c.Password == "" {
		return c, errors.New("ENERGYTHING_USERNAME and ENERGYTHING_PASSWORD are required")
	}
	if strings.Contains(c.Username, ":") {
		return c, errors.New("ENERGYTHING_USERNAME must not contain a colon")
	}
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || n < 1 || n > 65535 || (host != "" && net.ParseIP(host) == nil && host != "localhost") {
		return c, errors.New("ENERGYTHING_HTTP_ADDR must contain an IP address or localhost and a port from 1 to 65535")
	}
	c.Timezone, err = time.LoadLocation(value("ENERGYTHING_TIMEZONE", "Europe/London"))
	if err != nil {
		return c, errors.New("ENERGYTHING_TIMEZONE must be a valid IANA timezone")
	}
	c.ShutdownTimeout, err = time.ParseDuration(value("ENERGYTHING_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || c.ShutdownTimeout <= 0 {
		return c, errors.New("ENERGYTHING_SHUTDOWN_TIMEOUT must be a positive duration")
	}
	c.HistoryStart, err = time.Parse("2006-01-02", value("ENERGYTHING_HISTORY_START", "2022-01-01"))
	if err != nil || c.HistoryStart.Year() < 1 || c.HistoryStart.After(time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)) {
		return c, errors.New("ENERGYTHING_HISTORY_START must be YYYY-MM-DD and not in the future")
	}
	c.Octopus = Provider{APIKey: get("OCTOPUS_API_KEY"), Account: get("OCTOPUS_ACCOUNT_NUMBER"), ImportMPAN: get("OCTOPUS_IMPORT_MPAN"), ImportSerial: get("OCTOPUS_IMPORT_SERIAL"), ExportMPAN: get("OCTOPUS_EXPORT_MPAN"), ExportSerial: get("OCTOPUS_EXPORT_SERIAL")}
	c.Growatt = Provider{Token: get("GROWATT_API_TOKEN"), PlantID: get("GROWATT_PLANT_ID"), DeviceSerial: get("GROWATT_DEVICE_SERIAL")}
	configure := func(p *Provider, prefix string, configured, partial bool) {
		p.State = "disabled"
		p.Message = "Unconfigured. Set " + prefix + " credentials to enable this source."
		fail := func(message string) { p.State = "error"; p.Message = message }
		if configured {
			p.State = "valid"
			p.Message = "Configured. Automatic collection starts after discovery."
		}
		if partial {
			fail(prefix + " credentials are incomplete; configure all required credentials.")
		}
		duration := func(key, def string) time.Duration {
			d, e := time.ParseDuration(value(key, def))
			if e != nil || d <= 0 {
				fail(key + " must be a positive duration.")
			}
			return d
		}
		if prefix == "OCTOPUS" {
			p.RefreshInterval = duration("OCTOPUS_REFRESH_INTERVAL", "1h")
		} else {
			p.LatestInterval = duration("GROWATT_LATEST_INTERVAL", "5m")
			p.HistoryInterval = duration("GROWATT_HISTORY_INTERVAL", "1h")
			enabled, e := strconv.ParseBool(value("GROWATT_AGGREGATE_DAILY_CAP_ENABLED", "false"))
			if e != nil {
				fail("GROWATT_AGGREGATE_DAILY_CAP_ENABLED must be a boolean.")
			} else {
				p.AggregateDailyCapEnabled = enabled
			}
		}
		if !configured && !partial {
			for _, suffix := range []string{"IMPORT_MPAN", "IMPORT_SERIAL", "EXPORT_MPAN", "EXPORT_SERIAL", "PLANT_ID", "DEVICE_SERIAL"} {
				if get(prefix+"_"+suffix) != "" {
					fail(prefix + " settings require credentials.")
				}
			}
		}
	}
	configure(&c.Octopus, "OCTOPUS", c.Octopus.APIKey != "" && c.Octopus.Account != "", (c.Octopus.APIKey == "") != (c.Octopus.Account == ""))
	configure(&c.Growatt, "GROWATT", c.Growatt.Token != "", false)
	return c, nil
}
