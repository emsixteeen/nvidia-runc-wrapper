package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/google/uuid"
 )

var (
	AppName = "nvidia-runc-wrapper"
	AppVersion = "0.1"
	AppGitCommit = ""
)

const (
	NVIDIARuntime = "nvidia-container-runtime"
)

type args struct {
	bundleDirPath string
	cmd           string
}

func exitOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("ERROR: %s: %s: %v\n", os.Args[0], msg, err)
	}
}

func getArgs() (*args, error) {
	args := &args{}

	for i, param := range os.Args {
		if param == "--bundle" || param == "-b" {
			if len(os.Args) - i <= 1 {
				return nil, fmt.Errorf("bundle option needs an argument")
			}
			args.bundleDirPath = os.Args[i + 1]
		} else if param == "create" {
			args.cmd = param
		} else if param == "--wrapper-version" {
			args.cmd = "version"
		}
	}

	if args.bundleDirPath == "" {
		bundleDirPath, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("os.Getwd", err)
		}

		args.bundleDirPath = bundleDirPath
	}

	return args, nil
}

func execNVIDIARunc() {
	runcPath, err := exec.LookPath(NVIDIARuntime)
	exitOnError(err, fmt.Sprintf("cannot locate %s on PATH", NVIDIARuntime))

	err = syscall.Exec(runcPath, append([]string{runcPath}, os.Args[1:]...), os.Environ())
	exitOnError(err, fmt.Sprintf("cannot invoke %s", NVIDIARuntime))
}

func mutateNVIDIASettings(spec *specs.Spec) {
	if spec.Process == nil {
		return
	}

	var newEnv []string

	for _, v := range spec.Process.Env {
		if strings.HasPrefix(v, "NVIDIA_VISIBLE_DEVICES") == false {
			newEnv = append(newEnv, v)
			continue
		}

		kv := strings.Split(v, "=")
		if len(kv) != 2 {
			continue
		}

		val := kv[1]
		if strings.HasPrefix(val, "GPU-") == false {
			continue
		}

		gpus := strings.Split(val, ",")
		valid := true

		for _, gpu := range gpus {
			id := strings.Split(gpu, "GPU-")
			if len(id) != 2 {
				valid = false
				break
			}

			if _, err := uuid.Parse(id[1]); err != nil {
				valid = false
				break
			}
		}

		if valid == true {
			newEnv = append(newEnv, v)
		}
	}

	spec.Process.Env = newEnv
}

func main() {
	args, err := getArgs()
	exitOnError(err, "cannot get args")

	if args.cmd == "version" {
		fmt.Printf("%s version %s\n", AppName, AppVersion)
		fmt.Printf("commit: %s\n", AppGitCommit)
		fmt.Printf("spec: %s\n", specs.Version)
		fmt.Printf("execve: %s\n", NVIDIARuntime)

		return
	}

	if args.cmd != "create" {
		execNVIDIARunc()
		log.Fatalf("ERROR: %s: failed to invoke runc", os.Args[0])
	}

	jsonFile, err := os.OpenFile(args.bundleDirPath + "/config.json", os.O_RDWR, 0644)
	exitOnError(err, "open OCI file")
	defer jsonFile.Close()

	jsonContent, err := ioutil.ReadAll(jsonFile)
	exitOnError(err, "failed to read OCI file")

	var spec specs.Spec
	err = json.Unmarshal(jsonContent, &spec)
	exitOnError(err, "failed to parse JSON")

	mutateNVIDIASettings(&spec)
	jsonOutput, err := json.Marshal(spec)
	exitOnError(err, "failed to marshal JSON")

	err = jsonFile.Truncate(0)
	exitOnError(err, "truncating OCI file")

	_, err = jsonFile.WriteAt(jsonOutput, 0)
	exitOnError(err, "writing OCI file")

	execNVIDIARunc()
}
