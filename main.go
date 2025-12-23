package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Global   Global   `yaml:"global"`
	Clients  []Client `yaml:"clients"`
}

type Defaults struct {
	Duration    string `yaml:"duration"`
	Version     int    `yaml:"version"`
	CaCrt       string `yaml:"ca_crt"`
	CaKey       string `yaml:"ca_key"`
	OutCrt      string `yaml:"out_crt"`
	OutKey      string `yaml:"out_key"`
	SignCmdPost string `yaml:"sign_cmd_post"`
}
type Global struct {
	CmdPre  string `yaml:"cmd_pre"`
	CmdPost string `yaml:"cmd_post"`
}

type Client struct {
	Name        string   `yaml:"name"`
	IP          string   `yaml:"ip"`
	Networks    []string `yaml:"networks"`
	Groups      []string `yaml:"groups"`
	Duration    string   `yaml:"duration"`
	Version     int      `yaml:"version"`
	CaCrt       string   `yaml:"ca_crt"`
	CaKey       string   `yaml:"ca_key"`
	OutCrt      string   `yaml:"out_crt"`
	OutKey      string   `yaml:"out_key"`
	SignCmdPost string   `yaml:"sign_cmd_post"`
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func fileRemove(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func runCommand(cmdStr string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func applyPlaceholders(template string, values map[string]string) string {
	result := template
	for key, val := range values {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result
}

func main() {
	configPath := flag.String(
		"config",
		"config.yml",
		"Path to clients configuration file",
	)
	flag.Parse()

	caPass := os.Getenv("NEBULA_CA_PASSPHRASE")

	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("failed to read config %s: %v", *configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("invalid YAML in %s: %v", *configPath, err)
	}

	if len(cfg.Clients) == 0 {
		log.Fatal("no clients defined in config")
	}

	if cfg.Defaults.Duration == "" {
		cfg.Defaults.Duration = "0"
	}

	if cfg.Defaults.Version == 0 {
		cfg.Defaults.Version = 0
	}

	if cfg.Defaults.CaCrt == "" {
		cfg.Defaults.CaCrt = "ca.crt"
	}

	if cfg.Defaults.CaKey == "" {
		cfg.Defaults.CaKey = "ca.key"
	}

	if cfg.Defaults.OutCrt == "" {
		cfg.Defaults.OutCrt = "hosts/{{name}}.crt"
	}

	if cfg.Defaults.OutKey == "" {
		cfg.Defaults.OutKey = "hosts/{{name}}.key"
	}

	placeholders := map[string]string{
		"ca_crt": cfg.Defaults.CaCrt,
		"ca_key": cfg.Defaults.CaKey,
	}

	if cfg.Global.CmdPre != "" {
		cmdPre := applyPlaceholders(cfg.Global.CmdPre, placeholders)

		//log.Printf("Running cmd_pre: %s", cmdPre)
		if err := runCommand(cmdPre); err != nil {
			log.Fatalf("cmd_pre failed %v", err)
		}
	}

	for _, c := range cfg.Clients {
		if c.Name == "" || len(c.Networks) <= 0 {
			log.Fatalf("client name and ip are required: %+v", c)
		}

		placeholders := map[string]string{
			"name": c.Name,
		}

		duration := c.Duration
		if duration == "" {
			duration = cfg.Defaults.Duration
		}

		version := c.Version
		if version == 0 {
			version = cfg.Defaults.Version
		}

		caCrt := c.CaCrt
		if caCrt == "" {
			caCrt = cfg.Defaults.CaCrt
		}
		if ok, _ := fileExists(caCrt); !ok {
			log.Fatalf("CA crt file %s does not exist", caCrt)
		}

		caKey := c.CaKey
		if caKey == "" {
			caKey = cfg.Defaults.CaKey
		}
		if ok, _ := fileExists(caKey); !ok {
			log.Fatalf("CA key file %s does not exist", caKey)
		}

		outCrt := c.OutCrt
		if outCrt == "" {
			outCrt = cfg.Defaults.OutCrt
		}
		outCrt = applyPlaceholders(outCrt, placeholders)
		if ok, _ := fileExists(outCrt); ok {
			err := fileRemove(outCrt)
			if err != nil {
				return
			}
		}

		outKey := c.OutKey
		if outKey == "" {
			outKey = cfg.Defaults.OutKey
		}
		outKey = applyPlaceholders(outKey, placeholders)
		if ok, _ := fileExists(outKey); ok {
			err := fileRemove(outKey)
			if err != nil {
				return
			}
		}

		args := []string{
			"sign",
			"-name", c.Name,
			"-ip", c.IP,
			"-duration", duration,
			"-ca-crt", caCrt,
			"-ca-key", caKey,
			"-out-crt", outCrt,
			"-out-key", outKey,
			"-version", strconv.Itoa(version),
		}

		if len(c.Networks) > 0 {
			args = append(args, "-networks", strings.Join(c.Networks, ","))
		}

		if len(c.Groups) > 0 {
			args = append(args, "-groups", strings.Join(c.Groups, ","))
		}

		cmd := exec.Command("nebula-cert", args...)
		cmd.Stdin = bytes.NewBufferString(caPass + "\n")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fmt.Printf("→ Generating cert for %s\n", c.Name)
		if err := cmd.Run(); err != nil {
			log.Fatalf("failed to generate cert for %s: %v", c.Name, err)
		}

		signCmdPost := c.SignCmdPost
		if signCmdPost == "" {
			signCmdPost = cfg.Defaults.SignCmdPost
		}

		if signCmdPost != "" {
			cmdPost := applyPlaceholders(signCmdPost, placeholders)

			//log.Printf("Running cmd_post for %s: %s", c.Name, cmdPost)
			if err := runCommand(cmdPost); err != nil {
				log.Fatalf("cmd_post failed for %s: %v", c.Name, err)
			}
		}
	}

	if cfg.Global.CmdPost != "" {
		cmdPost := applyPlaceholders(cfg.Global.CmdPost, placeholders)

		//log.Printf("Running cmd_post for %s: %s", c.Name, cmdPost)
		if err := runCommand(cmdPost); err != nil {
			log.Fatalf("cmd_pre failed %v", err)
		}
	}

	fmt.Println("✔ All certificates generated successfully")
}
