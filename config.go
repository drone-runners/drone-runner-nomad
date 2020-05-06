// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config provides the runner configuration.
type Config struct {
	Debug   bool   `envconfig:"DRONE_DEBUG"`
	Trace   bool   `envconfig:"DRONE_TRACE"`
	Machine string `envconfig:"DRONE_MACHINE"`

	Job struct {
		Datacenter []string          `envconfig:"DRONE_JOB_DATACENTER" default:"dc1"`
		Namespace  string            `envconfig:"DRONE_JOB_NAMESPACE"`
		Region     string            `envconfig:"DRONE_JOB_REGION"`
		Prefix     string            `envconfig:"DRONE_JOB_PREFIX" default:"drone-job-"`
		Labels     map[string]string `envconfig:"DRONE_JOB_LABELS"`
	}

	Task struct {
		Compute int `envconfig:"DRONE_TASK_COMPUTE" default:"500"`
		Memory  int `envconfig:"DRONE_TASK_MEMORY"  default:"1024"`
	}

	Image struct {
		Name string `envconfig:"DRONE_IMAGE"      default:"drone/drone-runner-docker:latest"`
		Pull bool   `envconfig:"DRONE_IMAGE_PULL" default:"false"`
	}

	Server struct {
		Addr       string `envconfig:"-"`
		Proto      string `envconfig:"DRONE_RPC_PROTO"  required:"true" default:"http"`
		Host       string `envconfig:"DRONE_RPC_HOST"   required:"true"`
		Secret     string `envconfig:"DRONE_RPC_SECRET" required:"true"`
		SkipVerify bool   `envconfig:"DRONE_RPC_SKIP_VERIFY"`
		Dump       bool   `envconfig:"DRONE_RPC_DUMP_HTTP"`
		DumpBody   bool   `envconfig:"DRONE_RPC_DUMP_HTTP_BODY"`
	}

	Callback struct {
		Proto string `envconfig:"DRONE_CALLBACK_PROTO"`
		Host  string `envconfig:"DRONE_CALLBACK_HOST"`
	}

	// Environment is a collection of all DRONE_ environment
	// variables that are passed to the runner.
	Environ map[string]string `envconfig:"-"`
}

// legacy environment variables. the key is the legacy
// variable name, and the value is the new variable name.
var legacy = map[string]string{
	"DRONE_HOSTNAME":          "DRONE_MACHINE",
	"DRONE_NOMAD_DATACENTER":  "DRONE_JOB_DATACENTER",
	"DRONE_NOMAD_NAMESPACE":   "DRONE_JOB_NAMESPACE",
	"DRONE_NOMAD_REGION":      "DRONE_JOB_REGION",
	"DRONE_NOMAD_IMAGE":       "DRONE_IMAGE",
	"DRONE_NOMAD_IMAGE_PULL":  "DRONE_IMAGE_PULL",
	"DRONE_NOMAD_DEFAULT_RAM": "DRONE_TASK_MEMORY",
	"DRONE_NOMAD_DEFAULT_CPU": "DRONE_TASK_COMPUTE",
	"DRONE_NOMAD_LABELS":      "DRONE_JOB_LABELS",
	"DRONE_NOMAD_JOB_PREFIX":  "DRONE_JOB_PREFIX",
}

// ignore these variables when recording all DRONE_
// environment variables to pass to the docker runner.
var ignore = map[string]struct{}{
	"DRONE_JOB_DATACENTER":   struct{}{},
	"DRONE_JOB_NAMESPACE":    struct{}{},
	"DRONE_JOB_REGION":       struct{}{},
	"DRONE_JOB_PREFIX":       struct{}{},
	"DRONE_JOB_LABELS":       struct{}{},
	"DRONE_TASK_COMPUTE":     struct{}{},
	"DRONE_TASK_MEMORY":      struct{}{},
	"DRONE_MACHINE":          struct{}{},
	"DRONE_IMAGE":            struct{}{},
	"DRONE_IMAGE_PULL":       struct{}{},
	"DRONE_IMAGE_ENTRYPOINT": struct{}{},
	"DRONE_IMAGE_COMMAND":    struct{}{},
	"DRONE_IMAGE_ARGS":       struct{}{},
}

// Load loads the configuration from the environment.
func Load() (Config, error) {
	// loop through legacy environment variable and, if set
	// rewrite to the new variable name.
	for k, v := range legacy {
		if s, ok := os.LookupEnv(k); ok {
			os.Setenv(v, s)
		}
	}

	var config Config
	err := envconfig.Process("", &config)
	if err != nil {
		return config, err
	}

	if config.Machine == "" {
		config.Machine, _ = os.Hostname()
	}

	config.Server.Addr = fmt.Sprintf(
		"%s://%s",
		config.Server.Proto,
		config.Server.Host,
	)

	// loop through all environment variables and copy
	// those with a DRONE_ prefix to send to the docker
	// pipeline runner.
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "DRONE_") {
			continue
		}

		s := strings.SplitN(env, "=", 2)
		k := s[0]
		v := s[1]

		// ignore environment variables that are found
		// in the ignore list.
		if _, ok := ignore[k]; ok {
			continue
		}

		if config.Environ == nil {
			config.Environ = map[string]string{}
		}
		config.Environ[k] = v
	}

	// WARNING
	// this is intended for local development only and
	// instructs the docker runner to use a different server
	// url than the nomad runner.
	if v := config.Callback.Host; v != "" {
		config.Environ["DRONE_RPC_HOST"] = v
	}
	if v := config.Callback.Proto; v != "" {
		config.Environ["DRONE_RPC_PROTO"] = v
	}

	return config, nil
}
