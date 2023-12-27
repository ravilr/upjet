// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/muvaf/typewriter/pkg/wrapper"
	"github.com/pkg/errors"

	"github.com/crossplane/upjet/pkg/pipeline/templates"
)

var (
	regexTypeFile = regexp.MustCompile(`zz_(.+)_types.go`)
)

// NewConversionConvertibleGenerator returns a new ConversionConvertibleGenerator.
func NewConversionConvertibleGenerator(pkg *types.Package, rootDir, group, version string) *ConversionConvertibleGenerator {
	return &ConversionConvertibleGenerator{
		LocalDirectoryPath: filepath.Join(rootDir, "apis", strings.ToLower(strings.Split(group, ".")[0])),
		LicenseHeaderPath:  filepath.Join(rootDir, "hack", "boilerplate.go.txt"),
		SpokeVersionsMap:   make(map[string][]string),
		pkg:                pkg,
		version:            version,
	}
}

// ConversionConvertibleGenerator generates conversion methods implementing the
// conversion.Convertible interface on the CRD structs.
type ConversionConvertibleGenerator struct {
	LocalDirectoryPath string
	LicenseHeaderPath  string
	SpokeVersionsMap   map[string][]string

	pkg     *types.Package
	version string
}

// Generate writes generated conversion.Convertible interface functions
func (cg *ConversionConvertibleGenerator) Generate(cfgs []*terraformedInput) error {
	entries, err := os.ReadDir(cg.LocalDirectoryPath)
	if err != nil {
		return errors.Wrapf(err, "cannot list the directory entries for the source folder %s while generating the conversion.Convertible interface functions", cg.LocalDirectoryPath)
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name() == cg.version {
			// we skip spoke generation for the current version as the assumption is
			// the current CRD version is the hub version.
			continue
		}
		trFile := wrapper.NewFile(cg.pkg.Path(), cg.pkg.Name(), templates.ConversionConvertibleTemplate,
			wrapper.WithGenStatement(GenStatement),
			wrapper.WithHeaderPath(cg.LicenseHeaderPath),
		)
		filePath := filepath.Join(cg.LocalDirectoryPath, e.Name(), "zz_generated.conversion.go")
		vars := map[string]any{
			"APIVersion": e.Name(),
		}

		var resources []map[string]any
		versionDir := filepath.Join(cg.LocalDirectoryPath, e.Name())
		files, err := os.ReadDir(versionDir)
		if err != nil {
			return errors.Wrapf(err, "cannot list the directory entries for the source folder %s while looking for the generated types", versionDir)
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			m := regexTypeFile.FindStringSubmatch(f.Name())
			if len(m) < 2 {
				continue
			}
			c := findKindTerraformedInput(cfgs, m[1])
			if c == nil {
				// type may not be available in the new version =>
				// no conversion is possible.
				continue
			}
			resources = append(resources, map[string]any{
				"CRD": map[string]string{
					"Kind": c.Kind,
				},
			})
			cg.SpokeVersionsMap[fmt.Sprintf("%s.%s", c.ShortGroup, c.Kind)] = append(cg.SpokeVersionsMap[c.Kind], filepath.Base(versionDir))
		}

		vars["Resources"] = resources
		if err := trFile.Write(filePath, vars, os.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot write the generated conversion Hub functions file %s", filePath)
		}
	}
	return nil
}

func findKindTerraformedInput(cfgs []*terraformedInput, name string) *terraformedInput {
	for _, c := range cfgs {
		if name == strings.ToLower(c.Kind) {
			return c
		}
	}
	return nil
}
