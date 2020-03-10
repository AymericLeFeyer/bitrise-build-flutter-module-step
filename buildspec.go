package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-tools/go-steputils/output"
	"github.com/bitrise-tools/go-steputils/tools"
	"github.com/kballard/go-shellquote"
	"github.com/ryanuber/go-glob"
)

type buildSpecification struct {
	displayName          string
	platformCmdFlag      string
	platformSelectors    []string
	outputPathPatterns   []string
	additionalParameters string
	projectLocation      string
}

func (spec buildSpecification) exportArtifacts(artifacts []string) error {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	switch spec.platformCmdFlag {
	case "aar":
		return spec.exportAndroidArtifacts(artifacts, deployDir)
	case "ios-framework":
		return spec.exportIOSFramework(artifacts, deployDir)
	case "web":
		return spec.exportWeb(artifacts, deployDir)
	default:
		return fmt.Errorf("unsupported platform for exporting artifacts: %s. Supported platforms: aar, ios-framework, web", spec.platformCmdFlag)
	}
}

func (spec buildSpecification) artifactPaths(outputPathPatterns []string, isDir bool) ([]string, error) {
	var paths []string
	for _, outputPathPattern := range outputPathPatterns {
		pths, err := findPaths(spec.projectLocation, outputPathPattern, isDir)
		if err != nil {
			return nil, err
		}
		paths = append(paths, pths...)
	}
	return paths, nil
}

func (spec buildSpecification) exportIOSFramework(artifacts []string, deployDir string) error {
	artifact := artifacts[len(artifacts)-1]
	fileName := filepath.Base(artifact)

	if len(artifacts) > 1 {
		log.Warnf("- Multiple artifacts found: %v, exporting %s", artifacts, artifact)
	}

	if err := ziputil.ZipDir(artifact, filepath.Join(deployDir, fileName+".zip"), false); err != nil {
		return err
	}
	log.Donef("- $BITRISE_DEPLOY_DIR/" + fileName + ".zip")

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_APP_DIR_PATH", artifact); err != nil {
		return err
	}
	log.Donef("- $BITRISE_APP_DIR_PATH: " + artifact)

	return nil
}

func (spec buildSpecification) exportAndroidArtifacts(artifacts []string, deployDir string) error {

	var singleFileOutputEnvName string
	var multipleFileOutputEnvName string
	
	singleFileOutputEnvName = "BITRISE_AAR_PATH"
	multipleFileOutputEnvName = "BITRISE_AAR_PATH_LIST"

	var deployedFiles []string
	for _, path := range artifacts {
		deployedFilePath := filepath.Join(deployDir, filepath.Base(path))

		if err := output.ExportOutputFile(path, deployedFilePath, singleFileOutputEnvName); err != nil {
			return err
		}
		deployedFiles = append(deployedFiles, deployedFilePath)
	}
	if err := tools.ExportEnvironmentWithEnvman(multipleFileOutputEnvName, strings.Join(deployedFiles, "\n")); err != nil {
		return fmt.Errorf("failed to export enviroment variable %s, error: %s", multipleFileOutputEnvName, err)
	}

	log.Donef("- " + singleFileOutputEnvName + ": " + deployedFiles[len(deployedFiles)-1])
	log.Donef("- " + multipleFileOutputEnvName + ": " + strings.Join(deployedFiles, "|"))
	return nil
}

func (spec buildSpecification) exportWeb(artifacts []string, deployDir string) error {
	if len(artifacts) < 1 {
		failf("No artifact found")
	}

	var singleFileOutputEnvName string
	var multipleFileOutputEnvName string
	
	singleFileOutputEnvName = "BITRISE_WEB_DIRECTORY_PATH"
	multipleFileOutputEnvName = "BITRISE_WEB_DIRECTORY_PATH"

	var deployedFiles []string
	for _, path := range artifacts {
		deployedFilePath := filepath.Join(deployDir, filepath.Base(path))

		if err := output.ExportOutputFile(path, deployedFilePath, singleFileOutputEnvName); err != nil {
			return err
		}
		deployedFiles = append(deployedFiles, deployedFilePath)
	}
	if err := tools.ExportEnvironmentWithEnvman(multipleFileOutputEnvName, strings.Join(deployedFiles, "\n")); err != nil {
		return fmt.Errorf("failed to export enviroment variable %s, error: %s", multipleFileOutputEnvName, err)
	}

	log.Donef("- " + singleFileOutputEnvName + ": " + deployedFiles[len(deployedFiles)-1])
	log.Donef("- " + multipleFileOutputEnvName + ": " + strings.Join(deployedFiles, "|"))
	return nil
}

func (spec buildSpecification) buildable(platform string) bool {
	return sliceutil.IsStringInSlice(platform, spec.platformSelectors)
}

func findPaths(location string, outputPathPattern string, dir bool) (out []string, err error) {
	err = filepath.Walk(location, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !info.IsDir() == dir || !glob.Glob(outputPathPattern, path) {
			return nil
		}

		out = append(out, path)
		return nil
	})
	if len(out) == 0 && err == nil {
		log.Debugf("couldn't find output artifact on path: " + filepath.Join(location, outputPathPattern))
	}
	return
}

func (spec buildSpecification) build(params string) error {
	paramSlice, err := shellquote.Split(params)
	if err != nil {
		return err
	}

	var errorWriter io.Writer = os.Stderr
	var errBuffer bytes.Buffer

	buildCmd := command.New("flutter", append([]string{"build", spec.platformCmdFlag}, paramSlice...)...).SetStdout(os.Stdout)

	if spec.platformCmdFlag == "ios-framework" {
		buildCmd.SetStdin(strings.NewReader("a")) // if the CLI asks to input the selected identity we force it to be aborted
		errorWriter = io.MultiWriter(os.Stderr, &errBuffer)
	}

	buildCmd.SetStderr(errorWriter)

	fmt.Println()
	log.Donef("$ %s", buildCmd.PrintableCommandArgs())
	fmt.Println()

	buildCmd.SetDir(spec.projectLocation)

	err = buildCmd.Run()

	return err
}
