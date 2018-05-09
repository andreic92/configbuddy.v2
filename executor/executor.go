package executor

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/andreic92/configbuddy.v2/backup"
	"github.com/andreic92/configbuddy.v2/model"
	"github.com/andreic92/configbuddy.v2/parser"
	"github.com/ghodss/yaml"

	log "github.com/sirupsen/logrus"
)

type applicationExecutor struct {
	configs       *model.Arguments
	parser        parser.Parser
	finalConf     *model.ConfigWrapper
	backupService backup.BackupService
}

func StartConfiguring(config *model.Arguments, parse parser.Parser, backupService backup.BackupService) (err error) {
	executor := &applicationExecutor{configs: config, parser: parse, backupService: backupService}
	err = executor.readConfigs()
	if err != nil {
		return
	}

	err = executor.executePackages()
	if err != nil {
		return
	}

	err = executor.executeFiles()
	if err != nil {
		return
	}
	return nil
}

func (a *applicationExecutor) readConfigs() (err error) {
	if len(a.configs.Configs) == 0 {
		log.Infof("No config files provided. Nothing to do here. Exit...")
		return
	}

	var cfg *model.ConfigWrapper
	for _, filePath := range a.configs.Configs {
		cfg, err = loadConfig(cfg, filePath)
		if err != nil {
			log.WithError(err).Errorf("Error during validate %s", filePath)
			return
		}
	}

	a.finalConf = cfg
	return
}

func (a *applicationExecutor) executePackages() (err error) {
	return
}

func (a *applicationExecutor) executeFiles() (err error) {
	for name, act := range a.finalConf.Config.FileActions {
		fileExecutor, err := NewFileExecutor(&act, name, a.configs, a.parser, a.backupService)
		if err != nil {
			log.WithError(err).WithField("file action", act).Error("Error during processing fileAction")
			continue
		}
		err = fileExecutor.Execute()
		if err != nil {
			log.WithError(err).WithField("file action", act).Error("Error during processing fileAction")
		}
	}
	return
}

func loadConfig(appendToThis *model.ConfigWrapper, fileToLoad string) (*model.ConfigWrapper, error) {
	cfg, err := readFile(fileToLoad)
	if err != nil {
		return nil, err
	}
	if appendToThis == nil {
		appendToThis = cfg
		err = appendActionsToGlobalConfig(cfg, appendToThis)
		if err != nil {
			return nil, err
		}
	} else {
		err = appendActionsToGlobalConfig(cfg, appendToThis)
		if err != nil {
			return nil, err
		}
	}

	for _, includeFile := range cfg.Config.Includes {
		log.WithField("file", includeFile).Debug("include config")
		_, err := loadConfig(appendToThis, cfg.ConfigFileDirectory+"/"+includeFile)
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func appendActionsToGlobalConfig(cfg *model.ConfigWrapper, appendToThis *model.ConfigWrapper) error {
	// file actions
	if appendToThis.Config.FileActions == nil {
		appendToThis.Config.FileActions = make(map[string]model.FileAction)
	}
	for key, val := range cfg.Config.FileActions {
		abs, err := filepath.Abs(cfg.ConfigFileDirectory + "/" + val.Source)
		if err != nil {
			return err
		}
		val.Source = abs
		if strings.HasPrefix(val.Destination, ".") { // if the destination path is relative
			val.Destination = cfg.ConfigFileDirectory + "/" + val.Destination
		}
		appendToThis.Config.FileActions[key] = val
	}

	// package actions
	if appendToThis.Config.PackageActions == nil {
		appendToThis.Config.PackageActions = make(map[string]model.PackageAction)
	}
	for key, val := range cfg.Config.PackageActions {
		appendToThis.Config.PackageActions[key] = val
	}
	return nil
}

func readFile(filePath string) (*model.ConfigWrapper, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	log.WithField("file", abs).Debug("reading file")
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	var c model.Config
	err = yaml.Unmarshal(bytes, &c)
	if err != nil {
		return nil, err
	}
	return &model.ConfigWrapper{
		Config:              &c,
		ConfigFilePath:      abs,
		ConfigFileDirectory: filepath.Dir(abs),
	}, nil
}
