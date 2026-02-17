package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/depgraph"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	sbomFormat            string
	sbomOutput            string
	sbomIncludeTransitive bool
	sbomRegistryFlag      string
)

const (
	sbomFormatSPDX      = "spdx"
	sbomFormatCycloneDX = "cyclonedx"
)

var sbomCmd = &cobra.Command{
	Use:   "sbom",
	Short: "Generate Software Bill of Materials",
	Long: `Generate a Software Bill of Materials (SBOM) for your Bazel module.

This command parses your MODULE.bazel and generates an SBOM in standard formats
containing all direct and transitive dependencies.

Supported formats:
  - spdx (default): SPDX 2.3 JSON format
  - cyclonedx: CycloneDX 1.4 JSON format

Examples:
  bz sbom
  bz sbom --format=cyclonedx
  bz sbom --output=sbom.json
  bz sbom --include-transitive=false`,
	RunE:          runSBOM,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureSBOMCmd() {
	sbomCmd.Flags().StringVar(&sbomFormat, "format", sbomFormatSPDX, "Output format (spdx, cyclonedx)")
	sbomCmd.Flags().StringVar(&sbomOutput, "output", "", "Output file path (default: stdout)")
	sbomCmd.Flags().BoolVar(&sbomIncludeTransitive, "include-transitive", true, "Include transitive dependencies")
	sbomCmd.Flags().StringVar(&sbomRegistryFlag, "registry", "", "Registry URL (https://, http://, file://, or /path)")
	rootCmd.AddCommand(sbomCmd)
}

// SPDX 2.3 Types

// SPDXDocument represents an SPDX 2.3 document.
type SPDXDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      SPDXCreationInfo   `json:"creationInfo"`
	Packages          []SPDXPackage      `json:"packages"`
	Relationships     []SPDXRelationship `json:"relationships,omitempty"`
}

// SPDXCreationInfo contains creation information for the SPDX document.
type SPDXCreationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

// SPDXPackage represents a package in the SPDX document.
type SPDXPackage struct {
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	VersionInfo      string            `json:"versionInfo"`
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	LicenseConcluded string            `json:"licenseConcluded,omitempty"`
	LicenseDeclared  string            `json:"licenseDeclared,omitempty"`
	CopyrightText    string            `json:"copyrightText,omitempty"`
	ExternalRefs     []SPDXExternalRef `json:"externalRefs,omitempty"`
}

// SPDXExternalRef represents an external reference in SPDX.
type SPDXExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

// SPDXRelationship represents a relationship between SPDX elements.
type SPDXRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// CycloneDX 1.4 Types

// CycloneDXBOM represents a CycloneDX 1.4 BOM.
type CycloneDXBOM struct {
	BOMFormat    string                `json:"bomFormat"`
	SpecVersion  string                `json:"specVersion"`
	SerialNumber string                `json:"serialNumber,omitempty"`
	Version      int                   `json:"version"`
	Metadata     *CycloneDXMetadata    `json:"metadata,omitempty"`
	Components   []CycloneDXComponent  `json:"components"`
	Dependencies []CycloneDXDependency `json:"dependencies,omitempty"`
}

// CycloneDXMetadata contains metadata for the CycloneDX BOM.
type CycloneDXMetadata struct {
	Timestamp string              `json:"timestamp,omitempty"`
	Tools     []CycloneDXTool     `json:"tools,omitempty"`
	Component *CycloneDXComponent `json:"component,omitempty"`
}

// CycloneDXTool represents a tool used to create the BOM.
type CycloneDXTool struct {
	Vendor  string `json:"vendor,omitempty"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// CycloneDXComponent represents a component in the CycloneDX BOM.
type CycloneDXComponent struct {
	Type     string             `json:"type"`
	Name     string             `json:"name"`
	Version  string             `json:"version"`
	PURL     string             `json:"purl,omitempty"`
	BOMRef   string             `json:"bom-ref,omitempty"`
	Licenses []CycloneDXLicense `json:"licenses,omitempty"`
}

// CycloneDXLicense represents a license in CycloneDX format.
type CycloneDXLicense struct {
	License CycloneDXLicenseInfo `json:"license"`
}

// CycloneDXLicenseInfo contains license details.
type CycloneDXLicenseInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// CycloneDXDependency represents a dependency relationship.
type CycloneDXDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

// sbomDependency represents a dependency for SBOM generation.
type sbomDependency struct {
	Name             string
	Version          string
	DownloadLocation string
	License          string
	DirectDeps       []string // Names of direct dependencies
}

func runSBOM(cmd *cobra.Command, args []string) error {
	if err := cli.CheckCommandAllowed("sbom"); err != nil {
		return err
	}

	// Validate format
	format := strings.ToLower(sbomFormat)
	if format != sbomFormatSPDX && format != sbomFormatCycloneDX {
		return fmt.Errorf("unknown format: %s (valid formats: spdx, cyclonedx)", sbomFormat)
	}

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	ctx := cmdContext(cmd)

	registryURL := sbomRegistryFlag
	if registryURL == "" {
		registryURL = cli.GetRegistry()
	}
	if registryURL == "" {
		registryURL = registry.DefaultBCR
	}

	reg, err := registry.New(registryURL)
	if err != nil {
		return fmt.Errorf("invalid registry: %w", err)
	}

	// Collect dependencies
	deps := make(map[string]*sbomDependency)

	if sbomIncludeTransitive {
		// Collect all transitive dependencies
		visited := make(map[string]bool)
		for _, dep := range f.Deps {
			name := dep.Name.String()
			ver := dep.Version.String()
			collectTransitiveDeps(ctx, reg, registryURL, name, ver, deps, visited)
		}
	} else {
		// Only direct dependencies
		for _, dep := range f.Deps {
			name := dep.Name.String()
			ver := dep.Version.String()
			key := name + "@" + ver
			deps[key] = &sbomDependency{
				Name:             name,
				Version:          ver,
				DownloadLocation: buildDownloadLocation(registryURL, name, ver),
			}
		}
	}

	// Determine output writer
	var out io.Writer
	if sbomOutput != "" {
		file, err := os.Create(sbomOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		out = file
	} else {
		out = cmd.OutOrStdout()
	}

	// Generate SBOM
	switch format {
	case sbomFormatSPDX:
		return generateSPDX(out, f, deps)
	case sbomFormatCycloneDX:
		return generateCycloneDX(out, f, deps)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// collectTransitiveDeps recursively collects all transitive dependencies.
func collectTransitiveDeps(
	ctx context.Context,
	reg registry.Registry,
	registryURL, name, ver string,
	deps map[string]*sbomDependency,
	visited map[string]bool,
) {
	key := name + "@" + ver
	if visited[key] {
		return
	}
	visited[key] = true

	dep := &sbomDependency{
		Name:             name,
		Version:          ver,
		DownloadLocation: buildDownloadLocation(registryURL, name, ver),
	}
	deps[key] = dep

	depRefs, err := depgraph.FetchDeps(ctx, reg, depgraph.ModuleRef{Name: name, Version: ver})
	if err != nil {
		return
	}

	// Record direct deps of this module
	for _, depRef := range depRefs {
		depName := depRef.Name
		depVer := depRef.Version
		dep.DirectDeps = append(dep.DirectDeps, depName+"@"+depVer)
		collectTransitiveDeps(ctx, reg, registryURL, depName, depVer, deps, visited)
	}
}

// buildDownloadLocation constructs a download URL for a module.
func buildDownloadLocation(registryURL, name, ver string) string {
	// Handle file:// URLs
	if strings.HasPrefix(registryURL, "file://") || strings.HasPrefix(registryURL, "/") {
		return fmt.Sprintf("%s/modules/%s/%s/source.tar.gz", registryURL, name, ver)
	}

	// For BCR, use the canonical URL format
	if strings.Contains(registryURL, "bcr.bazel.build") || registryURL == registry.DefaultBCR {
		return fmt.Sprintf("https://bcr.bazel.build/modules/%s/%s/source.tar.gz", name, ver)
	}

	// Generic HTTP/HTTPS registry
	return fmt.Sprintf("%s/modules/%s/%s/source.tar.gz", strings.TrimSuffix(registryURL, "/"), name, ver)
}

// generateSPDX generates an SPDX 2.3 document.
func generateSPDX(w io.Writer, f *module.File, deps map[string]*sbomDependency) error {
	moduleName := f.Name()
	if moduleName == "" {
		moduleName = "unknown"
	}

	// Generate document namespace
	namespace := fmt.Sprintf("https://spdx.org/spdxdocs/%s-%s-%s",
		moduleName,
		f.Version(),
		uuid.New().String(),
	)

	doc := SPDXDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              moduleName,
		DocumentNamespace: namespace,
		CreationInfo: SPDXCreationInfo{
			Created:  time.Now().UTC().Format(time.RFC3339),
			Creators: []string{"Tool: bz"},
		},
		Packages:      make([]SPDXPackage, 0, len(deps)),
		Relationships: make([]SPDXRelationship, 0),
	}

	// Add version to tool creator if available
	if version != "" && version != "dev" {
		doc.CreationInfo.Creators = []string{fmt.Sprintf("Tool: bz-%s", version)}
	}

	// Add packages for each dependency
	for _, dep := range deps {
		spdxID := fmt.Sprintf("SPDXRef-Package-%s-%s", sanitizeSPDXID(dep.Name), sanitizeSPDXID(dep.Version))

		pkg := SPDXPackage{
			SPDXID:           spdxID,
			Name:             dep.Name,
			VersionInfo:      dep.Version,
			DownloadLocation: dep.DownloadLocation,
			FilesAnalyzed:    false,
			ExternalRefs: []SPDXExternalRef{
				{
					ReferenceCategory: "PACKAGE-MANAGER",
					ReferenceType:     "purl",
					ReferenceLocator:  fmt.Sprintf("pkg:bazel/%s@%s", dep.Name, dep.Version),
				},
			},
		}

		if dep.License != "" {
			pkg.LicenseConcluded = dep.License
		} else {
			pkg.LicenseConcluded = "NOASSERTION"
		}
		pkg.CopyrightText = "NOASSERTION"

		doc.Packages = append(doc.Packages, pkg)

		// Add DESCRIBES relationship from document to package
		doc.Relationships = append(doc.Relationships, SPDXRelationship{
			SPDXElementID:      "SPDXRef-DOCUMENT",
			RelationshipType:   "DESCRIBES",
			RelatedSPDXElement: spdxID,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// generateCycloneDX generates a CycloneDX 1.4 document.
func generateCycloneDX(w io.Writer, f *module.File, deps map[string]*sbomDependency) error {
	moduleName := f.Name()
	if moduleName == "" {
		moduleName = "unknown"
	}

	// Generate serial number
	serialNumber := fmt.Sprintf("urn:uuid:%s", uuid.New().String())

	bom := CycloneDXBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.4",
		SerialNumber: serialNumber,
		Version:      1,
		Metadata: &CycloneDXMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools: []CycloneDXTool{
				{
					Vendor:  "bz",
					Name:    "bz",
					Version: version,
				},
			},
			Component: &CycloneDXComponent{
				Type:    "application",
				Name:    moduleName,
				Version: f.Version(),
				BOMRef:  fmt.Sprintf("%s@%s", moduleName, f.Version()),
				PURL:    fmt.Sprintf("pkg:bazel/%s@%s", moduleName, f.Version()),
			},
		},
		Components:   make([]CycloneDXComponent, 0, len(deps)),
		Dependencies: make([]CycloneDXDependency, 0),
	}

	// Add components for each dependency
	for _, dep := range deps {
		bomRef := fmt.Sprintf("%s@%s", dep.Name, dep.Version)

		comp := CycloneDXComponent{
			Type:    "library",
			Name:    dep.Name,
			Version: dep.Version,
			PURL:    fmt.Sprintf("pkg:bazel/%s@%s", dep.Name, dep.Version),
			BOMRef:  bomRef,
		}

		if dep.License != "" {
			comp.Licenses = []CycloneDXLicense{
				{
					License: CycloneDXLicenseInfo{
						ID: dep.License,
					},
				},
			}
		}

		bom.Components = append(bom.Components, comp)

		// Add dependency relationship
		cdxDep := CycloneDXDependency{
			Ref:       bomRef,
			DependsOn: dep.DirectDeps,
		}
		bom.Dependencies = append(bom.Dependencies, cdxDep)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(bom)
}

// sanitizeSPDXID removes or replaces characters invalid in SPDX IDs.
func sanitizeSPDXID(s string) string {
	// SPDX IDs must contain only letters, numbers, dots, and hyphens
	result := strings.Builder{}
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			result.WriteRune(r)
		case r >= '0' && r <= '9':
			result.WriteRune(r)
		case r == '.' || r == '-':
			result.WriteRune(r)
		case r == '_':
			result.WriteRune('-')
		default:
			result.WriteRune('-')
		}
	}
	return result.String()
}
