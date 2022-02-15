package dotnetcoreaspnet_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	dotnetcoreaspnet "github.com/paketo-buildpacks/dotnet-core-aspnet"
	"github.com/paketo-buildpacks/dotnet-core-aspnet/fakes"
	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir         string
		workingDir        string
		cnbDir            string
		entryResolver     *fakes.EntryResolver
		dependencyManager *fakes.DependencyManager
		symlinker         *fakes.Symlinker
		clock             chronos.Clock
		timeStamp         time.Time
		buffer            *bytes.Buffer

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = ioutil.TempDir("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = ioutil.TempDir("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		entryResolver = &fakes.EntryResolver{}
		entryResolver.ResolveCall.Returns.BuildpackPlanEntry = packit.BuildpackPlanEntry{
			Name: "dotnet-aspnetcore",
			Metadata: map[string]interface{}{
				"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
				"version":        "2.5.x",
			},
		}

		dependencyManager = &fakes.DependencyManager{}
		dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{
			ID:   "dotnet-aspnetcore",
			Name: "Dotnet Core ASPNet",
		}
		dependencyManager.GenerateBillOfMaterialsCall.Returns.BOMEntrySlice = []packit.BOMEntry{
			{
				Name: "dotnet-aspnetcore",
				Metadata: packit.BOMMetadata{
					Version: "dotnet-aspnetcore-dep-version",
					Checksum: packit.BOMChecksum{
						Algorithm: packit.SHA256,
						Hash:      "dotnet-aspnetcore-dep-sha",
					},
					URI: "dotnet-aspnetcore-dep-uri",
				},
			},
		}

		symlinker = &fakes.Symlinker{}

		buffer = bytes.NewBuffer(nil)
		logEmitter := dotnetcoreaspnet.NewLogEmitter(buffer)

		timeStamp = time.Now()
		clock = chronos.NewClock(func() time.Time {
			return timeStamp
		})

		build = dotnetcoreaspnet.Build(entryResolver, dependencyManager, symlinker, logEmitter, clock)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	it("returns a result that builds correctly", func() {
		result, err := build(packit.BuildContext{
			WorkingDir: workingDir,
			CNBPath:    cnbDir,
			Stack:      "some-stack",
			BuildpackInfo: packit.BuildpackInfo{
				Name:    "Some Buildpack",
				Version: "some-version",
			},
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name: "dotnet-aspnetcore",
						Metadata: map[string]interface{}{
							"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
							"version":        "2.5.x",
						},
					},
				},
			},
			Layers: packit.Layers{Path: layersDir},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(result).To(Equal(packit.BuildResult{
			Layers: []packit.Layer{
				{
					Name: "dotnet-core-aspnet",
					Path: filepath.Join(layersDir, "dotnet-core-aspnet"),
					SharedEnv: packit.Environment{
						"DOTNET_ROOT.override": filepath.Join(workingDir, ".dotnet_root"),
					},
					LaunchEnv:        packit.Environment{},
					BuildEnv:         packit.Environment{},
					ProcessLaunchEnv: map[string]packit.Environment{},
					Build:            false,
					Launch:           false,
					Cache:            false,
					Metadata: map[string]interface{}{
						"dependency-sha": "",
						"built_at":       timeStamp.Format(time.RFC3339Nano),
					},
				},
			},
		}))

		Expect(dependencyManager.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
		Expect(dependencyManager.ResolveCall.Receives.Id).To(Equal("dotnet-aspnetcore"))
		Expect(dependencyManager.ResolveCall.Receives.Version).To(Equal("2.5.x"))
		Expect(dependencyManager.ResolveCall.Receives.Stack).To(Equal("some-stack"))

		Expect(dependencyManager.GenerateBillOfMaterialsCall.Receives.Dependencies).To(Equal([]postal.Dependency{
			{
				ID:   "dotnet-aspnetcore",
				Name: "Dotnet Core ASPNet",
			},
		}))

		Expect(dependencyManager.InstallCall.Receives.Dependency).To(Equal(postal.Dependency{ID: "dotnet-aspnetcore", Name: "Dotnet Core ASPNet"}))
		Expect(dependencyManager.InstallCall.Receives.CnbPath).To(Equal(cnbDir))
		Expect(dependencyManager.InstallCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "dotnet-core-aspnet")))

		Expect(symlinker.LinkCall.CallCount).To(Equal(1))
		Expect(symlinker.LinkCall.Receives.WorkingDir).To(Equal(workingDir))
		Expect(symlinker.LinkCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "dotnet-core-aspnet")))
	})

	context("when the 'RUNTIME_VERSION' env variable is set", func() {
		it.Before(func() {
			Expect(os.Setenv("RUNTIME_VERSION", "some-version")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("RUNTIME_VERSION")).To(Succeed())
		})

		it("doesnt call the entry resolver", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Stack:      "some-stack",
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "some-version",
				},
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: map[string]interface{}{
								"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
								"version":        "2.5.x",
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name: "dotnet-core-aspnet",
						Path: filepath.Join(layersDir, "dotnet-core-aspnet"),
						SharedEnv: packit.Environment{
							"DOTNET_ROOT.override": filepath.Join(workingDir, ".dotnet_root"),
						},
						LaunchEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            false,
						Launch:           false,
						Cache:            false,
						Metadata: map[string]interface{}{
							"dependency-sha": "",
							"built_at":       timeStamp.Format(time.RFC3339Nano),
						},
					},
				},
			}))

			Expect(entryResolver.ResolveCall.Receives.BuildpackPlanEntrySlice).To(ContainElement(packit.BuildpackPlanEntry{
				Name: "dotnet-aspnetcore",
				Metadata: map[string]interface{}{
					"version":        "some-version",
					"version-source": "RUNTIME_VERSION",
				},
			}))
		})
	})

	context("when the build plan entry include build, launch flags", func() {
		it.Before(func() {
			entryResolver.ResolveCall.Returns.BuildpackPlanEntry = packit.BuildpackPlanEntry{
				Name: "dotnet-aspnetcore",
				Metadata: map[string]interface{}{
					"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
					"version":        "2.5.x",
					"build":          true,
					"launch":         true,
				},
			}

			entryResolver.MergeLayerTypesCall.Returns.Launch = true
			entryResolver.MergeLayerTypesCall.Returns.Build = true
		})

		it("marks the layer as build, cache and launch", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Stack:      "some-stack",
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "some-version",
				},
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: map[string]interface{}{
								"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
								"version":        "2.5.x",
								"build":          true,
								"launch":         true,
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name: "dotnet-core-aspnet",
						Path: filepath.Join(layersDir, "dotnet-core-aspnet"),
						SharedEnv: packit.Environment{
							"DOTNET_ROOT.override": filepath.Join(workingDir, ".dotnet_root"),
						},
						LaunchEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Launch:           true,
						Cache:            true,
						Metadata: map[string]interface{}{
							"dependency-sha": "",
							"built_at":       timeStamp.Format(time.RFC3339Nano),
						},
					},
				},
				Build: packit.BuildMetadata{
					BOM: []packit.BOMEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: packit.BOMMetadata{
								Version: "dotnet-aspnetcore-dep-version",
								Checksum: packit.BOMChecksum{
									Algorithm: packit.SHA256,
									Hash:      "dotnet-aspnetcore-dep-sha",
								},
								URI: "dotnet-aspnetcore-dep-uri",
							},
						},
					},
				},
				Launch: packit.LaunchMetadata{
					BOM: []packit.BOMEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: packit.BOMMetadata{
								Version: "dotnet-aspnetcore-dep-version",
								Checksum: packit.BOMChecksum{
									Algorithm: packit.SHA256,
									Hash:      "dotnet-aspnetcore-dep-sha",
								},
								URI: "dotnet-aspnetcore-dep-uri",
							},
						},
					},
				},
			}))
		})
	})

	context("when there is a dependency cache match", func() {
		it.Before(func() {
			err := ioutil.WriteFile(filepath.Join(layersDir, "dotnet-core-aspnet.toml"), []byte("[metadata]\ndependency-sha = \"some-sha\"\n"), 0600)
			Expect(err).NotTo(HaveOccurred())

			dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{
				ID:     "dotnet-aspnetcore",
				SHA256: "some-sha",
			}
			entryResolver.MergeLayerTypesCall.Returns.Launch = false
			entryResolver.MergeLayerTypesCall.Returns.Build = true
		})

		it("exits build process early", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Stack:      "some-stack",
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "some-version",
				},
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: map[string]interface{}{
								"version":        "2.5.x",
								"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name:             "dotnet-core-aspnet",
						Path:             filepath.Join(layersDir, "dotnet-core-aspnet"),
						SharedEnv:        packit.Environment{},
						LaunchEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Launch:           false,
						Cache:            true,
						Metadata: map[string]interface{}{
							"dependency-sha": "some-sha",
						},
					},
				},
				Build: packit.BuildMetadata{
					BOM: []packit.BOMEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: packit.BOMMetadata{
								Version: "dotnet-aspnetcore-dep-version",
								Checksum: packit.BOMChecksum{
									Algorithm: packit.SHA256,
									Hash:      "dotnet-aspnetcore-dep-sha",
								},
								URI: "dotnet-aspnetcore-dep-uri",
							},
						},
					},
				},
			}))

			Expect(dependencyManager.GenerateBillOfMaterialsCall.Receives.Dependencies).To(Equal([]postal.Dependency{
				{
					ID:     "dotnet-aspnetcore",
					SHA256: "some-sha",
				},
			}))

			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
			Expect(symlinker.LinkCall.Receives.WorkingDir).To(Equal(workingDir))
			Expect(symlinker.LinkCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "dotnet-core-aspnet")))

			Expect(dependencyManager.InstallCall.CallCount).To(Equal(0))

			Expect(buffer.String()).To(ContainSubstring("Some Buildpack some-version"))
			Expect(buffer.String()).To(ContainSubstring("Resolving Dotnet Core ASPNet version"))
			Expect(buffer.String()).To(ContainSubstring("Selected dotnet-aspnetcore version (using BP_DOTNET_FRAMEWORK_VERSION): "))
			Expect(buffer.String()).To(ContainSubstring("Reusing cached layer"))
			Expect(buffer.String()).ToNot(ContainSubstring("Executing build process"))
		})
	})

	context("when version-source of the selected entry is buildpack.yml", func() {
		it.Before(func() {
			entryResolver.ResolveCall.Returns.BuildpackPlanEntry = packit.BuildpackPlanEntry{
				Name: "dotnet-aspnetcore",
				Metadata: map[string]interface{}{
					"version-source": "buildpack.yml",
					"version":        "2.5.x",
				},
			}
		})
		it("chooses the specified version and emits a warning", func() {
			_, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Stack:      "some-stack",
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "0.1.2",
				},
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "dotnet-aspnetcore",
							Metadata: map[string]interface{}{
								"version-source": "buildpack.yml",
								"version":        "2.5.x",
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(buffer.String()).To(ContainSubstring("Some Buildpack 0.1.2"))
			Expect(buffer.String()).To(ContainSubstring("Resolving Dotnet Core ASPNet version"))
			Expect(buffer.String()).To(ContainSubstring("Selected dotnet-aspnetcore version (using buildpack.yml): "))
			// v1.0.0 because that's the next major after input version v0.1.2
			Expect(buffer.String()).To(ContainSubstring("WARNING: Setting the .NET Framework version through buildpack.yml will be deprecated soon in Dotnet Core ASPNet Buildpack v1.0.0."))
			Expect(buffer.String()).To(ContainSubstring("Please specify the version through the $BP_DOTNET_FRAMEWORK_VERSION environment variable instead. See docs for more information."))
			Expect(buffer.String()).To(ContainSubstring("Executing build process"))
			Expect(buffer.String()).To(ContainSubstring("Configuring environment"))
		})
	})

	context("failure cases", func() {
		context("when the dependency cannot be resolved", func() {
			it.Before(func() {
				dependencyManager.ResolveCall.Returns.Error = errors.New("failed to resolve dependency")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "dotnet-aspnetcore"},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError("failed to resolve dependency"))
			})
		})

		context("when the dotnet symlinker fails on a rebuild", func() {
			it.Before(func() {
				err := ioutil.WriteFile(filepath.Join(layersDir, "dotnet-core-aspnet.toml"), []byte("[metadata]\ndependency-sha = \"some-sha\"\n"), 0600)
				Expect(err).NotTo(HaveOccurred())

				dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{
					ID:     "dotnet-aspnetcore",
					SHA256: "some-sha",
				}

				symlinker.LinkCall.Returns.Err = errors.New("symlinker error")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "dotnet-aspnetcore",
								Metadata: map[string]interface{}{
									"version":        "2.5.x",
									"version-source": "BP_DOTNET_FRAMEWORK_VERSION",
								},
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError("symlinker error"))
			})
		})

		context("when the dotnet-core-aspnet layer cannot be retrieved", func() {
			it.Before(func() {
				err := ioutil.WriteFile(filepath.Join(layersDir, "dotnet-core-aspnet.toml"), nil, 0000)
				Expect(err).NotTo(HaveOccurred())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "dotnet-aspnetcore"},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata")))
			})
		})

		context("when the dotnet-core-aspnet layer cannot be reset", func() {
			it.Before(func() {
				Expect(os.MkdirAll(filepath.Join(layersDir, "dotnet-core-aspnet", "something"), os.ModePerm)).To(Succeed())
				Expect(os.Chmod(filepath.Join(layersDir, "dotnet-core-aspnet"), 0500)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(filepath.Join(layersDir, "dotnet-core-aspnet"), os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "dotnet-aspnetcore"},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError(ContainSubstring("could not remove file")))
			})
		})

		context("when the dependency cannot be installed", func() {
			it.Before(func() {
				dependencyManager.InstallCall.Returns.Error = errors.New("failed to install dependency")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "dotnet-aspnetcore"},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError("failed to install dependency"))
			})
		})

		context("when the dotnet symlinker fails", func() {
			it.Before(func() {
				symlinker.LinkCall.Returns.Err = errors.New("symlinker error")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "dotnet-aspnetcore"},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError("symlinker error"))
			})
		})
	})
}
