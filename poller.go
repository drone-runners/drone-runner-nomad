// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/drone/runner-go/client"

	"github.com/hashicorp/nomad/api"
	"github.com/sirupsen/logrus"
)

// Poller polls the server for pending pipelines and then
// schedules the pipelines on the Nomad cluster.
type Poller struct {
	drone  client.Client
	client *api.Client
	config Config
}

// FromConfig returns a new Nomad scheduler.
func FromConfig(conf Config) (*Poller, error) {
	config := api.DefaultConfig()
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &Poller{client: client, config: conf}, nil
}

func (p *Poller) start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p.do(ctx)
		}
	}
}

func (p *Poller) do(ctx context.Context) error {
	logrus.Debug("requesting pipeline from server")

	stage, err := p.drone.Request(ctx, &client.Filter{
		Kind: "pipeline",
		Type: "docker",
	})
	if err == context.Canceled || err == context.DeadlineExceeded {
		logrus.WithError(err).Trace("no pipeline returned")
		return nil
	}
	if err != nil {
		logrus.WithError(err).Error("cannot request pipeline")
		return err
	}

	logrus.
		WithField("stage.id", stage.ID).
		WithField("stage.number", stage.Number).
		WithField("stage.os", stage.OS).
		WithField("stage.arch", stage.Arch).
		Debug("accepting pipeline")

	stage.Machine = p.config.Machine
	err = p.drone.Accept(ctx, stage)
	if err != nil && err == client.ErrOptimisticLock {
		logrus.Debug("pipeline accepted by another runner")
		return nil
	}
	if err != nil {
		logrus.WithError(err).Error("cannot accept pipeline")
		return err
	}

	task := &api.Task{
		Name:      "stage",
		Driver:    "docker",
		Env:       p.config.Environ,
		Resources: &api.Resources{},
		Config: map[string]interface{}{
			"image":      p.config.Image.Name,
			"force_pull": p.config.Image.Pull,
			"volumes": []string{
				"/var/run/docker.sock:/var/run/docker.sock",
			},
			"entrypoint": p.config.Image.Entrypoint,
			"command":    p.config.Image.Command,
			"args":       p.config.Image.Args,
		},
	}

	if stage.OS == "windows" {
		task.Config["volumes"] = []string{
			"////./pipe/docker_engine:////./pipe/docker_engine",
		}
	}

	if i := p.config.Task.Compute; i != 0 {
		task.Resources.CPU = intToPtr(i)
	}
	if i := p.config.Task.Memory; i != 0 {
		task.Resources.MemoryMB = intToPtr(i)
	}

	name := random()
	job := &api.Job{
		ID:          stringToPtr(name),
		Name:        stringToPtr(name),
		Type:        stringToPtr("batch"),
		Datacenters: p.config.Job.Datacenter,
		TaskGroups: []*api.TaskGroup{
			&api.TaskGroup{
				Name:  stringToPtr("pipeline"),
				Tasks: []*api.Task{task},
				RestartPolicy: &api.RestartPolicy{
					Mode: stringToPtr("fail"),
				},
			},
		},
		Meta: map[string]string{
			"io.drone":                 "true",
			"io.drone.stage.created":   time.Unix(stage.Created, 0).String(),
			"io.drone.stage.scheduled": time.Now().String(),
			"io.drone.stage.id":        fmt.Sprint(stage.ID),
			"io.drone.stage.number":    fmt.Sprint(stage.Number),
			"io.drone.stage.kind":      fmt.Sprint(stage.Kind),
			"io.drone.stage.type":      fmt.Sprint(stage.Type),
			"io.drone.stage.os":        fmt.Sprint(stage.OS),
			"io.drone.stage.arch":      fmt.Sprint(stage.Arch),
			"io.drone.build.id":        fmt.Sprint(stage.BuildID),
		},
	}

	if s := p.config.Job.Namespace; s != "" {
		job.Namespace = stringToPtr(s)
	}
	if s := p.config.Job.Region; s != "" {
		job.Region = stringToPtr(s)
	}

	// if we are running on darwin we disable os and arch
	// constraints, since it is possible nomad is running
	// on the host machine and reports a darwin os, instead
	// of a linux os.
	if runtime.GOOS != "darwin" {
		job.Constraints = []*api.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: stage.OS,
				Operand: "=",
			},
			{
				LTarget: "${attr.cpu.arch}",
				RTarget: stage.Arch,
				Operand: "=",
			},
		}
	}

	for k, v := range stage.Labels {
		job.Constraints = append(job.Constraints, &api.Constraint{
			LTarget: fmt.Sprintf("${meta.%s}", k),
			RTarget: v,
			Operand: "=",
		})
	}

	for k, v := range p.config.Job.Labels {
		job.Constraints = append(job.Constraints, &api.Constraint{
			LTarget: fmt.Sprintf("${meta.%s}", k),
			RTarget: v,
			Operand: "=",
		})
	}

	logrus.Debug("creating nomad job")
	_, _, err = p.client.Jobs().RegisterOpts(job, &api.RegisterOptions{}, nil)
	if err != nil {
		logrus.WithError(err).
			Error("cannot create job")
	} else {
		logrus.WithField("job.id", job.ID).
			Debug("created nomad job")
	}

	return err
}
