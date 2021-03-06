// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

//go:generate pluginator
package main

import (
	"fmt"

	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

type plugin struct {
	loadedPatches []*resource.Resource
	Paths         []types.PatchStrategicMerge `json:"paths,omitempty" yaml:"paths,omitempty"`
	Patches       string                      `json:"patches,omitempty" yaml:"patches,omitempty"`
}

//noinspection GoUnusedGlobalVariable
var KustomizePlugin plugin

func (p *plugin) Config(
	h *resmap.PluginHelpers, c []byte) (err error) {
	err = yaml.Unmarshal(c, p)
	if err != nil {
		return err
	}
	if len(p.Paths) == 0 && p.Patches == "" {
		return fmt.Errorf("empty file path and empty patch content")
	}
	if len(p.Paths) != 0 {
		for _, onePath := range p.Paths {
			// The following oddly attempts to interpret a path string as an
			// actual patch (instead of as a path to a file containing a patch).
			// All tests pass if this code is commented out.  This code should
			// be deleted; the user should use the Patches field which
			// exists for this purpose (inline patch declaration).
			res, err := h.ResmapFactory().RF().SliceFromBytes([]byte(onePath))
			if err == nil {
				p.loadedPatches = append(p.loadedPatches, res...)
				continue
			}
			res, err = h.ResmapFactory().RF().SliceFromPatches(
				h.Loader(), []types.PatchStrategicMerge{onePath})
			if err != nil {
				return err
			}
			p.loadedPatches = append(p.loadedPatches, res...)
		}
	}
	if p.Patches != "" {
		res, err := h.ResmapFactory().RF().SliceFromBytes([]byte(p.Patches))
		if err != nil {
			return err
		}
		p.loadedPatches = append(p.loadedPatches, res...)
	}

	if len(p.loadedPatches) == 0 {
		return fmt.Errorf(
			"patch appears to be empty; files=%v, Patch=%s", p.Paths, p.Patches)
	}
	// Merge the patches, looking for conflicts.
	_, err = h.ResmapFactory().ConflatePatches(p.loadedPatches)
	if err != nil {
		return err
	}
	return nil
}

func (p *plugin) Transform(m resmap.ResMap) error {
	for _, patch := range p.loadedPatches {
		target, err := m.GetById(patch.OrgId())
		if err != nil {
			return err
		}
		if err = m.ApplySmPatch(
			resource.MakeIdSet([]*resource.Resource{target}), patch); err != nil {
			return err
		}
	}
	return nil
}
