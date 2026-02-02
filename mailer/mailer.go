package mailer

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	gomail "gopkg.in/gomail.v2"
)

var (
	cfg     smtpConfig
	cfgOnce sync.Once
)

// loadConfigFromFile loads SMTP config from given file path. If file missing or incomplete, it will not override values left empty.
func loadConfigFromFile(path string) (smtpConfig, error) {
	cfg := smtpConfig{Host: "smtp.163.com", Port: 465}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	lines := strings.Split(string(data), "\n")
	vals := map[string]string{}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, ";") {
			continue
		}
		parts := strings.SplitN(ln, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		vals[k] = v
	}
	if v, ok := vals["SMTP_HOST"]; ok && v != "" {
		cfg.Host = v
	}
	if v, ok := vals["SMTP_PORT"]; ok && v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v, ok := vals["SMTP_USER"]; ok && v != "" {
		cfg.User = v
	}
	if v, ok := vals["SMP_USER"]; ok && v != "" {
		cfg.User = v
	}
	if v, ok := vals["SMTP_PASS"]; ok && v != "" {
		cfg.Pass = v
	}
	if v, ok := vals["SMP_PASS"]; ok && v != "" {
		cfg.Pass = v
	}
	if v, ok := vals["SMTP_FROM"]; ok && v != "" {
		cfg.From = v
	}

	// fallback to env variables if still missing
	if cfg.Host == "" {
		if h := os.Getenv("SMTP_HOST"); h != "" {
			cfg.Host = h
		}
	}
	if cfg.Port == 0 {
		if p := os.Getenv("SMTP_PORT"); p != "" {
			if v, err := strconv.Atoi(p); err == nil {
				cfg.Port = v
			}
		}
	}
	if cfg.User == "" {
		if u := os.Getenv("SMTP_USER"); u != "" {
			cfg.User = u
		}
		if cfg.User == "" {
			if u := os.Getenv("SMP_USER"); u != "" {
				cfg.User = u
			}
		}
	}
	if cfg.Pass == "" {
		if p := os.Getenv("SMTP_PASS"); p != "" {
			cfg.Pass = p
		}
		if cfg.Pass == "" {
			if p := os.Getenv("SMP_PASS"); p != "" {
				cfg.Pass = p
			}
		}
	}
	if cfg.From == "" {
		if f := os.Getenv("SMTP_FROM"); f != "" {
			cfg.From = f
		}
		if cfg.From == "" {
			cfg.From = cfg.User
		}
	}
	return cfg, nil
}

// configForRecipient picks an SMTP configuration file based on recipient email domain.
// It tries multiple candidate file names and falls back to default conf/mail.ini.
func configForRecipient(to string) smtpConfig {
	// Always prefer the 163 SMTP config for sending verification codes.
	// If conf/mail-163.ini exists, load it; otherwise fall back to conf/mail.ini or environment.
	if c, err := loadConfigFromFile("conf/mail-163.ini"); err == nil {
		return c
	}

	// fallback to general mail.ini or defaults
	if c, err := loadConfigFromFile("conf/mail.ini"); err == nil {
		return c
	}

	cfgOnce.Do(loadConfig)
	return cfg
}

type smtpConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func loadConfig() {
	// defaults
	cfg = smtpConfig{Host: "smtp.163.com", Port: 465}

	// try read conf/mail.ini
	data, err := os.ReadFile("conf/mail.ini")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		vals := map[string]string{}
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, ";") {
				continue
			}
			parts := strings.SplitN(ln, "=", 2)
			if len(parts) != 2 {
				continue
			}
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			vals[k] = v
		}
		// support both SMTP_USER and SMP_USER (typo in some configs)
		if v, ok := vals["SMTP_HOST"]; ok && v != "" {
			cfg.Host = v
		}
		if v, ok := vals["SMTP_PORT"]; ok && v != "" {
			if p, err := strconv.Atoi(v); err == nil {
				cfg.Port = p
			}
		}
		if v, ok := vals["SMTP_USER"]; ok && v != "" {
			cfg.User = v
		}
		if v, ok := vals["SMP_USER"]; ok && v != "" {
			cfg.User = v
		}
		if v, ok := vals["SMTP_PASS"]; ok && v != "" {
			cfg.Pass = v
		}
		if v, ok := vals["SMP_PASS"]; ok && v != "" {
			cfg.Pass = v
		}
		if v, ok := vals["SMTP_FROM"]; ok && v != "" {
			cfg.From = v
		}
	}

	// fall back to environment variables if any missing
	if cfg.Host == "" {
		if h := os.Getenv("SMTP_HOST"); h != "" {
			cfg.Host = h
		}
	}
	if cfg.Port == 0 {
		if p := os.Getenv("SMTP_PORT"); p != "" {
			if v, err := strconv.Atoi(p); err == nil {
				cfg.Port = v
			}
		}
	}
	if cfg.User == "" {
		if u := os.Getenv("SMTP_USER"); u != "" {
			cfg.User = u
		}
		if cfg.User == "" {
			if u := os.Getenv("SMP_USER"); u != "" {
				cfg.User = u
			}
		}
	}
	if cfg.Pass == "" {
		if p := os.Getenv("SMTP_PASS"); p != "" {
			cfg.Pass = p
		}
		if cfg.Pass == "" {
			if p := os.Getenv("SMP_PASS"); p != "" {
				cfg.Pass = p
			}
		}
	}
	if cfg.From == "" {
		if f := os.Getenv("SMTP_FROM"); f != "" {
			cfg.From = f
		}
		if cfg.From == "" {
			cfg.From = cfg.User
		}
	}
}

// Send sends a plain text email using SMTP config selected by recipient domain.
// It looks for domain-specific config files (e.g., conf/mail-163.ini or conf/mail-qq.com) and
// falls back to conf/mail.ini or environment variables when needed.
func Send(to, subject, body string) error {
	c := configForRecipient(to)

	m := gomail.NewMessage()
	from := c.From
	if from == "" {
		from = c.User
	}
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := gomail.NewDialer(c.Host, c.Port, c.User, c.Pass)
	logrus.Infof("mailer: sending mail via %s:%d from %s to %s", c.Host, c.Port, from, to)
	if err := d.DialAndSend(m); err != nil {
		logrus.Errorf("mailer: send failed via %s:%d to %s: %v", c.Host, c.Port, to, err)
		return err
	}
	return nil
}
