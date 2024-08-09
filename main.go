package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/qauzy/mat/component/updater"
	"github.com/qauzy/mat/config"
	C "github.com/qauzy/mat/constant"
	"github.com/qauzy/mat/constant/features"
	"github.com/qauzy/mat/hub"
	"github.com/qauzy/mat/hub/executor"
	"github.com/qauzy/mat/log"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"go.uber.org/automaxprocs/maxprocs"
)

var (
	version                bool
	testConfig             bool
	geodataMode            bool
	homeDir                string
	configFile             string
	externalUI             string
	externalController     string
	externalControllerUnix string
	secret                 string

	access string
)

//go:embed all:tpl
var engine embed.FS

func init() {
	flag.StringVar(&homeDir, "d", os.Getenv("CLASH_HOME_DIR"), "set configuration directory")
	flag.StringVar(&configFile, "f", os.Getenv("CLASH_CONFIG_FILE"), "specify configuration file")
	flag.StringVar(&externalUI, "ext-ui", os.Getenv("CLASH_OVERRIDE_EXTERNAL_UI_DIR"), "override external ui directory")
	flag.StringVar(&externalController, "ext-ctl", os.Getenv("CLASH_OVERRIDE_EXTERNAL_CONTROLLER"), "override external controller address")
	flag.StringVar(&externalControllerUnix, "ext-ctl-unix", os.Getenv("CLASH_OVERRIDE_EXTERNAL_CONTROLLER_UNIX"), "override external controller unix address")
	flag.StringVar(&secret, "secret", os.Getenv("CLASH_OVERRIDE_SECRET"), "override secret for RESTful API")
	flag.BoolVar(&geodataMode, "m", false, "set geodata mode")
	flag.BoolVar(&version, "v", false, "show current version of mat")
	flag.BoolVar(&testConfig, "t", false, "test configuration and exit")

	flag.StringVar(&access, "a", "", "access token")
	flag.Parse()
}

func main() {
	_, _ = maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))
	if version {
		fmt.Printf("Mat Meta %s %s %s with %s %s\n",
			C.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), C.BuildTime)
		if tags := features.Tags(); len(tags) != 0 {
			fmt.Printf("Use tags: %s\n", strings.Join(tags, ", "))
		}

		return
	}

	if homeDir != "" {
		if !filepath.IsAbs(homeDir) {
			currentDir, _ := os.Getwd()
			homeDir = filepath.Join(currentDir, homeDir)
		}
		C.SetHomeDir(homeDir)
	}

	if access != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		C.SetHomeDir(filepath.Join(home, ".netat"))

		if !fileExists(filepath.Join(home, ".netat", "geoip.metadb")) {
			// If aria2c binary doesn't exist, extract it.
			if err := extractFile("tpl/geoip.metadb", filepath.Join(home, ".netat")); err != nil {
				fmt.Printf("Initial extractFile  error: %s", err.Error())
			}
		}

		content, err := engine.ReadFile("tpl/template.yml")
		if err != nil {
			fmt.Println("engine,err=", err)
			return
		}
		userConfigMap := make(map[string]interface{})
		err = yaml.Unmarshal(content, &userConfigMap)
		if err != nil {
			fmt.Println("yaml,err=", err)
			return
		}

		userConfigMap["access-token"] = access
		content, err = yaml.Marshal(userConfigMap)
		if err != nil {
			fmt.Println("yaml.Marshal,err=", err)
			return
		}
		logrus.Info("[config] config changed, reloading...")

		cfg, err := executor.ParseWithBytes(content)
		if err != nil {
			fmt.Println("executor.ParseWithPath err:", err)
			return
		}
		executor.ApplyConfig(cfg, true)

	}

	if configFile != "" {
		if !filepath.IsAbs(configFile) {
			currentDir, _ := os.Getwd()
			configFile = filepath.Join(currentDir, configFile)
		}
	} else {
		configFile = filepath.Join(C.Path.HomeDir(), C.Path.Config())
	}
	C.SetConfig(configFile)

	if geodataMode {
		C.GeodataMode = true
	}

	if err := config.Init(C.Path.HomeDir()); err != nil {
		log.Fatalln("Initial configuration directory error: %s", err.Error())
	}

	if testConfig {
		if _, err := executor.Parse(); err != nil {
			log.Errorln(err.Error())
			fmt.Printf("configuration file %s test failed\n", C.Path.Config())
			os.Exit(1)
		}
		fmt.Printf("configuration file %s test is successful\n", C.Path.Config())
		return
	}

	var options []hub.Option
	if externalUI != "" {
		options = append(options, hub.WithExternalUI(externalUI))
	}
	if externalController != "" {
		options = append(options, hub.WithExternalController(externalController))
	}
	if externalControllerUnix != "" {
		options = append(options, hub.WithExternalControllerUnix(externalControllerUnix))
	}
	if secret != "" {
		options = append(options, hub.WithSecret(secret))
	}

	if err := hub.Parse(options...); err != nil {
		log.Fatalln("Parse config error: %s", err.Error())
	}

	if C.GeoAutoUpdate {
		updater.RegisterGeoUpdater(func() {
			cfg, err := executor.ParseWithPath(C.Path.Config())
			if err != nil {
				log.Errorln("[GEO] update GEO databases failed: %v", err)
				return
			}

			log.Warnln("[GEO] update GEO databases success, applying config")

			executor.ApplyConfig(cfg, false)
		})
	}

	defer executor.Shutdown()

	termSign := make(chan os.Signal, 1)
	hupSign := make(chan os.Signal, 1)
	signal.Notify(termSign, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(hupSign, syscall.SIGHUP)
	for {
		select {
		case <-termSign:
			return
		case <-hupSign:
			if cfg, err := executor.ParseWithPath(C.Path.Config()); err == nil {
				executor.ApplyConfig(cfg, true)
			} else {
				log.Errorln("Parse config error: %s", err.Error())
			}
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractFile(src, dstDir string) error {
	content, err := engine.ReadFile(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstPath := filepath.Join(dstDir, filepath.Base(src))
	if err = os.WriteFile(dstPath, content, 0644); err != nil {
		return err
	}

	return nil
}
