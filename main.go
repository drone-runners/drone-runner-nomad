// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package main

import (
	"context"

	"github.com/drone/runner-go/client"
	"github.com/drone/runner-go/logger"
	"github.com/drone/signal"

	"github.com/hashicorp/nomad/api"
	_ "github.com/joho/godotenv/autoload"
	"github.com/sirupsen/logrus"
)

var noContext = context.Background()

func main() {
	config, err := Load()
	if err != nil {
		logrus.WithError(err).
			Fatalln("failed to load configuration")
	}

	//
	// setup the drone client
	//

	cli := client.New(
		config.Server.Addr,
		config.Server.Secret,
		config.Server.SkipVerify,
	)
	if config.Server.Dump {
		cli.Dumper = logger.StandardDumper(
			config.Server.DumpBody,
		)
	}
	cli.Logger = logger.Logrus(
		logrus.NewEntry(
			logrus.StandardLogger(),
		),
	)

	//
	// setup the logrus logger
	//

	logger.Default = logger.Logrus(
		logrus.NewEntry(
			logrus.StandardLogger(),
		),
	)

	switch {
	case config.Debug:
		logrus.SetLevel(logrus.DebugLevel)
	case config.Trace:
		logrus.SetLevel(logrus.TraceLevel)
	}

	//
	// setup the nomad client
	//

	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		logrus.WithError(err).
			Fatalln("failed to create the nomad client")
	}

	//
	// setup graceful shutdown
	//

	ctx, cancel := context.WithCancel(noContext)
	defer cancel()
	ctx = signal.WithContextFunc(ctx, func() {
		println("received signal, terminating process")
		cancel()
	})

	//
	// create and start the scheduler
	//
	s := &Poller{
		drone:  cli,
		client: client,
		config: config,
	}
	if err := s.start(ctx); err != nil {
		logrus.WithError(err).
			Errorln("shutting down the server")
	}
}
