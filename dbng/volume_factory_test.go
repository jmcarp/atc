package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeFactory", func() {
	var (
		dbConn           dbng.Conn
		volumeFactory    *dbng.VolumeFactory
		containerFactory *dbng.ContainerFactory
		teamFactory      dbng.TeamFactory
		buildFactory     *dbng.BuildFactory
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		containerFactory = dbng.NewContainerFactory(dbConn)
		volumeFactory = dbng.NewVolumeFactory(dbConn)
		teamFactory = dbng.NewTeamFactory(dbConn)
		buildFactory = dbng.NewBuildFactory(dbConn)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetOrphanedVolumes", func() {
		var (
			build                     *dbng.Build
			expectedCreatedHandles    []string
			expectedDestroyingHandles []string
		)

		BeforeEach(func() {
			team, err := teamFactory.CreateTeam("some-team")
			Expect(err).ToNot(HaveOccurred())

			build, err = buildFactory.CreateOneOffBuild(team)
			Expect(err).ToNot(HaveOccurred())

			workerFactory := dbng.NewWorkerFactory(dbConn)
			worker, err := workerFactory.SaveWorker(atc.Worker{
				Name:       "some-worker",
				GardenAddr: "1.2.3.4:7777",
			}, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			resourceCache := dbng.ResourceCache{
				ResourceConfig: dbng.ResourceConfig{
					CreatedByBaseResourceType: &dbng.BaseResourceType{
						Name: "some-resource-type",
					},
				},
			}
			usedResourceCache, err := resourceCache.FindOrCreateForBuild(setupTx, build)
			Expect(err).NotTo(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())

			creatingContainer, err := containerFactory.CreateTaskContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			expectedCreatedHandles = []string{}
			expectedDestroyingHandles = []string{}

			creatingVolume1, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingVolume3, err := createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())
			expectedDestroyingHandles = append(expectedDestroyingHandles, destroyingVolume3.Handle())

			resourceCacheVolume, err := volumeFactory.CreateResourceCacheVolume(team, worker, usedResourceCache)
			Expect(err).NotTo(HaveOccurred())

			_, err = resourceCacheVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			deleteTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			deleted, err := build.Delete(deleteTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			deleted, err = usedResourceCache.Destroy(deleteTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())
			Expect(deleteTx.Commit()).To(Succeed())

			createdContainer, err := creatingContainer.Created("some-handle")
			Expect(err).NotTo(HaveOccurred())
			destroyingContainer, err := createdContainer.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err := destroyingContainer.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(BeTrue())
		})

		It("returns orphaned volumes", func() {
			createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
			Expect(err).NotTo(HaveOccurred())
			createdHandles := []string{}
			for _, vol := range createdVolumes {
				createdHandles = append(createdHandles, vol.Handle())
			}
			Expect(createdHandles).To(Equal(expectedCreatedHandles))

			destoryingHandles := []string{}
			for _, vol := range destoryingVolumes {
				destoryingHandles = append(destoryingHandles, vol.Handle())
			}
			Expect(destoryingHandles).To(Equal(destoryingHandles))
		})
	})
})

// 	Describe("CreateWorkerResourceTypeVolume", func() {
// 		var worker dbng.Worker
// 		var wrt dbng.WorkerResourceType

// 		BeforeEach(func() {
// 			worker = dbng.Worker{
// 				Name:       "some-worker",
// 				GardenAddr: "1.2.3.4:7777",
// 			}

// 			wrt = dbng.WorkerResourceType{
// 				WorkerName: worker.Name,
// 				Type:       "some-worker-resource-type",
// 				Image:      "some-worker-resource-image",
// 				Version:    "some-worker-resource-version",
// 			}
// 		})

// 		Context("when the worker resource type exists", func() {
// 			BeforeEach(func() {
// 				setupTx, err := dbConn.Begin()
// 				Expect(err).ToNot(HaveOccurred())

// 				defer setupTx.Rollback()

// 				err = worker.Create(setupTx)
// 				Expect(err).ToNot(HaveOccurred())

// 				_, err = wrt.Create(setupTx)
// 				Expect(err).ToNot(HaveOccurred())

// 				Expect(setupTx.Commit()).To(Succeed())
// 			})

// 			It("returns the created volume", func() {
// 				volume, err := factory.CreateWorkerResourceTypeVolume(wrt)
// 				Expect(err).ToNot(HaveOccurred())
// 				Expect(volume.ID).ToNot(BeZero())
// 			})
// 		})

// 		Context("when the worker resource type does not exist", func() {
// 			It("returns ErrWorkerResourceTypeNotFound", func() {
// 				_, err := factory.CreateWorkerResourceTypeVolume(wrt)
// 				Expect(err).To(Equal(dbng.ErrWorkerResourceTypeNotFound))
// 			})
// 		})
// 	})
// })
