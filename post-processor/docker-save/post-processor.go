// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package dockersave

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-docker/builder/docker"
	dockerimport "github.com/hashicorp/packer-plugin-docker/post-processor/docker-import"
	dockertag "github.com/hashicorp/packer-plugin-docker/post-processor/docker-tag"
	"github.com/hashicorp/packer-plugin-sdk/common"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

const BuilderId = "packer.post-processor.docker-save"

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	Executable string `mapstructure:"docker_path"`
	Path       string `mapstructure:"path"`

	ctx interpolate.Context
}

type PostProcessor struct {
	Driver docker.Driver

	config Config
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec { return p.config.FlatMapstructure().HCL2Spec() }

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}

	if p.config.Executable == "" {
		p.config.Executable = "docker"
	}

	return nil

}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, artifact packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	if artifact.BuilderId() != dockerimport.BuilderId &&
		artifact.BuilderId() != dockertag.BuilderId {
		err := fmt.Errorf(
			"Unknown artifact type: %s\nCan only save Docker builder artifacts.",
			artifact.BuilderId())
		return nil, false, false, err
	}

	path := p.config.Path

	// Open the file that we're going to write to
	f, err := os.Create(path)
	if err != nil {
		err := fmt.Errorf("Error creating output file: %s", err)
		return nil, false, false, err
	}

	driver := p.Driver
	if driver == nil {
		// If no driver is set, then we use the real driver
		driver = &docker.DockerDriver{
			Executable: p.config.Executable,
			Ctx:        &p.config.ctx,
			Ui:         ui,
		}
	}

	ui.Message("Saving image: " + artifact.Id())

	if err := driver.SaveImage(artifact.Id(), f); err != nil {
		f.Close()
		os.Remove(f.Name())

		return nil, false, false, err
	}

	f.Close()
	ui.Message("Saved to: " + path)

	return artifact, true, false, nil
}
