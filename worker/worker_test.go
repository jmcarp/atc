package worker_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/baggageclaimfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker", func() {
	var (
		logger                 *lagertest.TestLogger
		fakeGardenClient       *gfakes.FakeClient
		fakeBaggageclaimClient *bfakes.FakeClient
		fakeVolumeClient       *wfakes.FakeVolumeClient
		fakeVolumeFactory      *wfakes.FakeVolumeFactory
		fakeImageFactory       *wfakes.FakeImageFactory
		fakeImage              *wfakes.FakeImage
		fakeGardenWorkerDB     *wfakes.FakeGardenWorkerDB
		fakeWorkerProvider     *wfakes.FakeWorkerProvider
		fakeClock              *fakeclock.FakeClock
		fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory
		fakeDBContainerFactory *wfakes.FakeDBContainerFactory
		activeContainers       int
		resourceTypes          []atc.WorkerResourceType
		platform               string
		tags                   atc.Tags
		teamID                 int
		workerName             string
		workerStartTime        int64
		httpProxyURL           string
		httpsProxyURL          string
		noProxy                string
		origUptime             time.Duration
		workerUptime           uint64

		gardenWorker Worker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeVolumeClient = new(wfakes.FakeVolumeClient)
		fakeVolumeFactory = new(wfakes.FakeVolumeFactory)
		fakeImageFactory = new(wfakes.FakeImageFactory)
		fakeImage = new(wfakes.FakeImage)
		fakeImageFactory.NewImageReturns(fakeImage)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeWorkerProvider = new(wfakes.FakeWorkerProvider)
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42
		resourceTypes = []atc.WorkerResourceType{
			{
				Type:    "some-resource",
				Image:   "some-resource-image",
				Version: "some-version",
			},
		}
		platform = "some-platform"
		tags = atc.Tags{"some", "tags"}
		teamID = 17
		workerName = "some-worker"
		workerStartTime = fakeClock.Now().Unix()
		workerUptime = 0

		fakeDBContainerFactory = new(wfakes.FakeDBContainerFactory)
	})

	JustBeforeEach(func() {
		gardenWorker = NewGardenWorker(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeVolumeClient,
			fakeVolumeFactory,
			fakeImageFactory,
			fakePipelineDBFactory,
			fakeDBContainerFactory,
			fakeGardenWorkerDB,
			fakeWorkerProvider,
			fakeClock,
			activeContainers,
			resourceTypes,
			platform,
			tags,
			teamID,
			workerName,
			"1.2.3.4",
			workerStartTime,
			httpProxyURL,
			httpsProxyURL,
			noProxy,
		)

		origUptime = gardenWorker.Uptime()
		fakeClock.IncrementBySeconds(workerUptime)
	})

	XDescribe("LookupContainer", func() {
		var handle string

		BeforeEach(func() {
			handle = "we98lsv"
		})

		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer  *gfakes.FakeContainer
				foundContainer Container
				findErr        error
				found          bool
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")
				fakeDBContainerFactory.FindContainerReturns(&dbng.CreatedContainer{}, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			JustBeforeEach(func() {
				foundContainer, found, findErr = gardenWorker.LookupContainer(logger, handle)
			})

			FIt("returns the container and no error", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Context("when the concourse:volumes property is present", func() {
				var (
					handle1Volume         *wfakes.FakeVolume
					handle2Volume         *wfakes.FakeVolume
					expectedHandle1Volume *wfakes.FakeVolume
					expectedHandle2Volume *wfakes.FakeVolume
				)

				BeforeEach(func() {
					handle1Volume = new(wfakes.FakeVolume)
					handle2Volume = new(wfakes.FakeVolume)
					expectedHandle1Volume = new(wfakes.FakeVolume)
					expectedHandle2Volume = new(wfakes.FakeVolume)

					fakeContainer.PropertiesReturns(garden.Properties{
						"concourse:volumes":       `["handle-1","handle-2"]`,
						"concourse:volume-mounts": `{"handle-1":"/handle-1/path","handle-2":"/handle-2/path"}`,
					}, nil)

					fakeBaggageclaimClient.LookupVolumeStub = func(logger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
						if handle == "handle-1" {
							return handle1Volume, true, nil
						} else if handle == "handle-2" {
							return handle2Volume, true, nil
						} else {
							panic("unknown handle: " + handle)
						}
					}

					fakeVolumeFactory.BuildWithIndefiniteTTLStub = func(logger lager.Logger, vol baggageclaim.Volume) (Volume, error) {
						if vol == handle1Volume {
							return expectedHandle1Volume, nil
						} else if vol == handle2Volume {
							return expectedHandle2Volume, nil
						} else {
							panic("unknown volume: " + vol.Handle())
						}
					}
				})

				Describe("VolumeMounts", func() {
					It("returns all bound volumes based on properties on the container", func() {
						Expect(foundContainer.VolumeMounts()).To(ConsistOf([]VolumeMount{
							{Volume: expectedHandle1Volume, MountPath: "/handle-1/path"},
							{Volume: expectedHandle2Volume, MountPath: "/handle-2/path"},
						}))
					})

					Context("when LookupVolume returns an error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
						})

						It("returns the error on lookup", func() {
							Expect(findErr).To(Equal(disaster))
						})
					})

					Context("when Build returns an error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVolumeFactory.BuildWithIndefiniteTTLReturns(nil, disaster)
						})

						It("returns the error on lookup", func() {
							Expect(findErr).To(Equal(disaster))
						})
					})
				})

				Describe("Release", func() {
					It("releases the container's volumes once and only once", func() {
						foundContainer.Release(FinalTTL(time.Minute))
						Expect(expectedHandle1Volume.ReleaseCallCount()).To(Equal(1))
						Expect(expectedHandle1Volume.ReleaseArgsForCall(0)).To(Equal(FinalTTL(time.Minute)))
						Expect(expectedHandle2Volume.ReleaseCallCount()).To(Equal(1))
						Expect(expectedHandle2Volume.ReleaseArgsForCall(0)).To(Equal(FinalTTL(time.Minute)))

						foundContainer.Release(FinalTTL(time.Hour))
						Expect(expectedHandle1Volume.ReleaseCallCount()).To(Equal(1))
						Expect(expectedHandle2Volume.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("when the user property is present", func() {
				var (
					actualSpec garden.ProcessSpec
					actualIO   garden.ProcessIO
				)

				BeforeEach(func() {
					actualSpec = garden.ProcessSpec{
						Path: "some-path",
						Args: []string{"some", "args"},
						Env:  []string{"some=env"},
						Dir:  "some-dir",
					}

					actualIO = garden.ProcessIO{}

					fakeContainer.PropertiesReturns(garden.Properties{"user": "maverick"}, nil)
				})

				JustBeforeEach(func() {
					foundContainer.Run(actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
							User: "maverick",
						}))
						Expect(io).To(Equal(garden.ProcessIO{}))
					})
				})
			})

			Context("when the user property is not present", func() {
				var (
					actualSpec garden.ProcessSpec
					actualIO   garden.ProcessIO
				)

				BeforeEach(func() {
					actualSpec = garden.ProcessSpec{
						Path: "some-path",
						Args: []string{"some", "args"},
						Env:  []string{"some=env"},
						Dir:  "some-dir",
					}

					actualIO = garden.ProcessIO{}

					fakeContainer.PropertiesReturns(garden.Properties{"user": ""}, nil)
				})

				JustBeforeEach(func() {
					foundContainer.Run(actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the default user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
							User: "root",
						}))
						Expect(io).To(Equal(garden.ProcessIO{}))
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
					})
				})
			})
		})

		Context("when the gardenClient returns garden.ContainerNotFoundError", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "some-handle"})
			})

			It("returns false and no error", func() {
				_, found, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).ToNot(HaveOccurred())

				Expect(found).To(BeFalse())
			})
		})

		Context("when the gardenClient returns an error", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("container not found")
				fakeGardenClient.LookupReturns(nil, expectedErr)
			})

			It("returns nil and forwards the error", func() {
				foundContainer, _, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).To(Equal(expectedErr))

				Expect(foundContainer).To(BeNil())
			})
		})
	})

	Describe("ValidateResourceCheckVersion", func() {
		var (
			container db.SavedContainer
			valid     bool
			checkErr  error
		)

		BeforeEach(func() {
			container = db.SavedContainer{
				Container: db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						ResourceTypeVersion: atc.Version{
							"custom-type": "some-version",
						},
						CheckType: "custom-type",
					},
					ContainerMetadata: db.ContainerMetadata{
						WorkerName: "some-worker",
					},
				},
			}
		})

		JustBeforeEach(func() {
			valid, checkErr = gardenWorker.ValidateResourceCheckVersion(container)
		})

		Context("when not a check container", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeTask,
						},
					},
				}
			})

			It("returns true", func() {
				Expect(valid).To(BeTrue())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when container version matches worker's", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-resource": "some-version",
							},
							CheckType: "some-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
						},
					},
				}
			})

			It("returns true", func() {
				Expect(valid).To(BeTrue())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when container version does not match worker's", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-resource": "some-other-version",
							},
							CheckType: "some-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
						},
					},
				}
			})

			It("returns false", func() {
				Expect(valid).To(BeFalse())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when worker does not provide version for the resource type", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-other-resource": "some-other-version",
							},
							CheckType: "some-other-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
						},
					},
				}
			})

			It("returns false", func() {
				Expect(valid).To(BeFalse())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when container belongs to pipeline", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-resource": "some-version",
							},
							CheckType: "some-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
							PipelineID: 1,
						},
					},
				}
			})

			Context("when failing to get pipeline from database", func() {
				BeforeEach(func() {
					fakeGardenWorkerDB.GetPipelineByIDReturns(db.SavedPipeline{}, errors.New("disaster"))
				})

				It("returns an error", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr.Error()).To(ContainSubstring("disaster"))
				})

			})

			Context("when pipeline was found", func() {
				var fakePipelineDB *dbfakes.FakePipelineDB
				BeforeEach(func() {
					fakePipelineDB = new(dbfakes.FakePipelineDB)
					fakePipelineDBFactory.BuildReturns(fakePipelineDB)
				})

				Context("resource type is not found", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceTypeReturns(db.SavedResourceType{}, false, nil)
					})

					Context("when worker version matches", func() {
						BeforeEach(func() {
							container.Container.ResourceTypeVersion["some-resource"] = "some-version"
						})

						It("returns true", func() {
							Expect(valid).To(BeTrue())
							Expect(checkErr).NotTo(HaveOccurred())
						})
					})

					Context("when worker version does not match", func() {
						BeforeEach(func() {
							container.Container.ResourceTypeVersion["some-resource"] = "some-other-version"
						})

						It("returns false", func() {
							Expect(valid).To(BeFalse())
							Expect(checkErr).NotTo(HaveOccurred())
						})
					})
				})

				Context("resource type is found", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceTypeReturns(db.SavedResourceType{}, true, nil)
					})

					It("returns true", func() {
						Expect(valid).To(BeTrue())
						Expect(checkErr).NotTo(HaveOccurred())
					})
				})

				Context("getting resource type fails", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceTypeReturns(db.SavedResourceType{}, false, errors.New("disaster"))
					})

					It("returns false and error", func() {
						Expect(valid).To(BeFalse())
						Expect(checkErr).To(HaveOccurred())
						Expect(checkErr.Error()).To(ContainSubstring("disaster"))
					})
				})
			})
		})

	})

	XDescribe("FindContainerForIdentifier", func() {
		var (
			id Identifier

			foundContainer Container
			found          bool
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{
				ResourceID: 1234,
			}
		})

		JustBeforeEach(func() {
			foundContainer, found, lookupErr = gardenWorker.FindContainerForIdentifier(logger, id)
		})

		Context("when the container can be found", func() {
			var (
				fakeContainer      *gfakes.FakeContainer
				fakeSavedContainer db.SavedContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeSavedContainer = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							CheckType:           "some-resource",
							ResourceTypeVersion: atc.Version{"some-resource": "some-version"},
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "provider-handle",
							WorkerName: "some-worker",
						},
					},
				}
				fakeWorkerProvider.FindContainerForIdentifierReturns(fakeSavedContainer, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
				fakeGardenWorkerDB.GetContainerReturns(fakeSavedContainer, true, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).NotTo(HaveOccurred())
			})

			It("looks for containers with matching properties via the Garden client", func() {
				Expect(fakeWorkerProvider.FindContainerForIdentifierCallCount()).To(Equal(1))
				Expect(fakeWorkerProvider.FindContainerForIdentifierArgsForCall(0)).To(Equal(id))

				Expect(fakeGardenClient.LookupCallCount()).To(Equal(1))
				lookupHandle := fakeGardenClient.LookupArgsForCall(0)
				Expect(lookupHandle).To(Equal("provider-handle"))
			})

			Context("when container is check container", func() {
				BeforeEach(func() {
					fakeSavedContainer.Type = db.ContainerTypeCheck
					fakeWorkerProvider.FindContainerForIdentifierReturns(fakeSavedContainer, true, nil)
				})

				Context("when container resource version matches worker resource version", func() {
					It("returns container", func() {
						Expect(found).To(BeTrue())
						Expect(foundContainer.Handle()).To(Equal("provider-handle"))
					})
				})

				Context("when container resource version does not match worker resource version", func() {
					BeforeEach(func() {
						fakeSavedContainer.ResourceTypeVersion = atc.Version{"some-resource": "some-other-version"}
						fakeWorkerProvider.FindContainerForIdentifierReturns(fakeSavedContainer, true, nil)
					})

					It("does not return container", func() {
						Expect(found).To(BeFalse())
						Expect(lookupErr).NotTo(HaveOccurred())
					})
				})
			})

			Describe("the found container", func() {
				It("can be destroyed", func() {
					err := foundContainer.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("destroying via garden")
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("provider-handle"))

					By("no longer heartbeating")
					fakeClock.Increment(30 * time.Second)
					Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
				})

				It("performs an initial heartbeat synchronously", func() {
					Expect(fakeContainer.SetGraceTimeCallCount()).To(Equal(1))
					Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).To(Equal(1))
				})

				Describe("every 30 seconds", func() {
					It("heartbeats to the database and the container", func() {
						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
						Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(5 * time.Minute))

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(5 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(3))
						Expect(fakeContainer.SetGraceTimeArgsForCall(2)).To(Equal(5 * time.Minute))

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(3))
						handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(2)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(5 * time.Minute))
					})
				})

				Describe("releasing", func() {
					It("sets a final ttl on the container and stops heartbeating", func() {
						foundContainer.Release(FinalTTL(30 * time.Minute))

						Expect(fakeContainer.SetGraceTimeCallCount()).Should(Equal(2))
						Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(30 * time.Minute))

						Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).Should(Equal(2))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(30 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
						Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
					})

					Context("with no final ttl", func() {
						It("does not perform a final heartbeat", func() {
							foundContainer.Release(nil)

							Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
							Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(1))
						})
					})
				})

				It("can be released multiple times", func() {
					foundContainer.Release(nil)

					Expect(func() {
						foundContainer.Release(nil)
					}).NotTo(Panic())
				})
			})
		})

		Context("when the container cannot be found", func() {
			BeforeEach(func() {
				containerToReturn := db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "handle",
						},
					},
				}

				fakeWorkerProvider.FindContainerForIdentifierReturns(containerToReturn, true, nil)
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "handle"})
			})

			It("expires the container and returns false and no error", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundContainer).To(BeNil())

				expiredHandle := fakeWorkerProvider.ReapContainerArgsForCall(0)
				Expect(expiredHandle).To(Equal("handle"))
			})
		})

		Context("when looking up the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				containerToReturn := db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "handle",
						},
					},
				}

				fakeWorkerProvider.FindContainerForIdentifierReturns(containerToReturn, true, nil)
				fakeGardenClient.LookupReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})
	})

	Describe("Satisfying", func() {
		var (
			spec WorkerSpec

			satisfyingWorker Worker
			satisfyingErr    error

			customTypes atc.ResourceTypes
		)

		BeforeEach(func() {
			spec = WorkerSpec{
				Tags:   []string{"some", "tags"},
				TeamID: teamID,
			}

			customTypes = atc.ResourceTypes{
				{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-a",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-c",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-d",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "unknown-custom-type",
					Type:   "unknown-base-type",
					Source: atc.Source{"some": "source"},
				},
			}
		})

		JustBeforeEach(func() {
			satisfyingWorker, satisfyingErr = gardenWorker.Satisfying(spec, customTypes)
		})

		Context("when the platform is compatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-platform"
			})

			Context("when no tags are specified", func() {
				BeforeEach(func() {
					spec.Tags = nil
				})

				It("returns ErrIncompatiblePlatform", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})

			Context("when the worker has no tags", func() {
				BeforeEach(func() {
					tags = []string{}
					spec.Tags = []string{}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the platform is incompatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-bogus-platform"
			})

			It("returns ErrIncompatiblePlatform", func() {
				Expect(satisfyingErr).To(Equal(ErrIncompatiblePlatform))
			})
		})

		Context("when the resource type is supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-resource"
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the resource type is a custom type supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "custom-type-c"
			})

			It("returns the worker", func() {
				Expect(satisfyingWorker).To(Equal(gardenWorker))
			})

			It("returns no error", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})
		})

		Context("when the resource type is a custom type that overrides one supported by the worker", func() {
			BeforeEach(func() {
				customTypes = append(customTypes, atc.ResourceType{
					Name:   "some-resource",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				})

				spec.ResourceType = "some-resource"
			})

			It("returns the worker", func() {
				Expect(satisfyingWorker).To(Equal(gardenWorker))
			})

			It("returns no error", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})
		})

		Context("when the resource type is a custom type that results in a circular dependency", func() {
			BeforeEach(func() {
				customTypes = append(customTypes, atc.ResourceType{
					Name:   "circle-a",
					Type:   "circle-b",
					Source: atc.Source{"some": "source"},
				}, atc.ResourceType{
					Name:   "circle-b",
					Type:   "circle-c",
					Source: atc.Source{"some": "source"},
				}, atc.ResourceType{
					Name:   "circle-c",
					Type:   "circle-a",
					Source: atc.Source{"some": "source"},
				})

				spec.ResourceType = "circle-a"
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the resource type is a custom type not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "unknown-custom-type"
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the type is not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-other-resource"
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when spec specifies team", func() {
			BeforeEach(func() {
				teamID = 123
				spec.TeamID = teamID
			})

			Context("when worker belongs to same team", func() {
				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when worker belongs to different team", func() {
				BeforeEach(func() {
					teamID = 777
				})

				It("returns ErrTeamMismatch", func() {
					Expect(satisfyingErr).To(Equal(ErrTeamMismatch))
				})
			})

			Context("when worker does not belong to any team", func() {
				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when spec does not specify a team", func() {
			Context("when worker belongs to no team", func() {
				BeforeEach(func() {
					teamID = 0
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when worker belongs to any team", func() {
				BeforeEach(func() {
					teamID = 555
				})

				It("returns ErrTeamMismatch", func() {
					Expect(satisfyingErr).To(Equal(ErrTeamMismatch))
				})
			})
		})
	})
})
