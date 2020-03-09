package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

// const ...
const (
)

var flutterConfigPath = filepath.Join(os.Getenv("HOME"), ".flutter_settings")

type config struct {
	Platform                string              `env:"platform,opt[both,ios,android,web]"`
	IOSExportPattern        string              `env:"ios_output_pattern,required"`
	AndroidExportPattern    string              `env:"android_output_pattern,required"`
	WebExportPattern    	string              `env:"web_output_pattern,required"`
	ProjectLocation         string              `env:"project_location,dir"`
	DebugMode               bool                `env:"is_debug_mode,opt[true,false]"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)
	log.SetEnableDebugLog(cfg.DebugMode)

	projectLocationAbs, err := filepath.Abs(cfg.ProjectLocation)
	if err != nil {
		failf("Failed to get absolute project path, error: %s", err)
	}

	exist, err := pathutil.IsDirExists(projectLocationAbs)
	if err != nil {
		failf("Failed to check if project path exists, error: %s", err)
	} else if !exist {
		failf("Project path does not exist.")
	}

	for _, spec := range []buildSpecification{
		{
			displayName:          "iOS",
			platformCmdFlag:      "ios-framework",
			platformSelectors:    []string{"both", "ios"},
			outputPathPatterns:   append(strings.Split(cfg.IOSExportPattern, "\n")),
		},
		{
			displayName:          "Android",
			platformCmdFlag:      "aar",
			platformSelectors:    []string{"both", "android"},
			outputPathPatterns:   append(strings.Split(cfg.AndroidExportPattern, "\n")),
		},
		{
			displayName:          "Web",
			platformCmdFlag:      "web",
			platformSelectors:    []string{"web"},
			outputPathPatterns:   append(strings.Split(cfg.WebExportPattern, "\n")),
		},
	} {
		if !spec.buildable(cfg.Platform) {
			continue
		}

		spec.projectLocation = projectLocationAbs

		fmt.Println()
		log.Infof("Build " + spec.displayName)
		if err := spec.build(spec.additionalParameters); err != nil {
			failf("Failed to build %s platform, error: %s", spec.displayName, err)
		}

		fmt.Println()
		log.Infof("Export " + spec.displayName + " artifact")

		var artifacts []string
		var err error

		if spec.platformCmdFlag == "aar" {
			artifacts, err = spec.artifactPaths(spec.outputPathPatterns, false)
		} else {
			artifacts, err = spec.artifactPaths(spec.outputPathPatterns, true)
		}
		if err != nil {
			failf("failed to find artifacts, error: %s", err)
		}

		log.Infof("will exportWeb")
		log.Infof("will exportWeb " + len(artifacts))

		if err := spec.exportArtifacts(artifacts); err != nil {
			failf("Failed to export %s artifacts, error: %s", spec.displayName, err)
		}
	}

	fmt.Println()
	log.Infof("Collecting cache")

	if err := cacheCocoapodsDeps(projectLocationAbs); err != nil {
		log.Warnf("Failed to collect cocoapods cache, error: %s", err)
	}

	if err := cacheCarthageDeps(projectLocationAbs); err != nil {
		log.Warnf("Failed to collect carthage cache, error: %s", err)
	}

	if err := cacheAndroidDeps(projectLocationAbs); err != nil {
		log.Warnf("Failed to collect android cache, error: %s", err)
	}

	if err := cacheFlutterDeps(projectLocationAbs); err != nil {
		log.Warnf("Failed to collect flutter cache, error: %s", err)
	}
}
