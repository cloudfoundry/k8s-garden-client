package integration_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("rep", func() {
	var (
		appGUID       string
		containerGUID string

		processGUID string
	)

	BeforeEach(func() {
		appGUID = uuid.NewString()
		containerGUID = uuid.NewString()
		processGUID = fmt.Sprintf("%s-%s", appGUID, containerGUID)
	})

	Describe("DesiredLRP", func() {
		AfterEach(func() {
			Expect(bbsClient.RemoveDesiredLRP(logger, "trace", processGUID)).To(Succeed())
		})

		It("Creates a Pod from a DesiredLRP", func() {
			By("Creating a DesiredLRP", func() {
				Expect(bbsClient.DesireLRP(logger, "trace", &models.DesiredLRP{
					ProcessGuid: processGUID,
					Domain:      "cf",
					RootFs:      "preloaded:cflinuxfs4",
					Instances:   1,
					DiskMb:      256,
					MemoryMb:    64,
					Network: &models.Network{
						Properties: map[string]string{
							"app_id":             appGUID,
							"container_workload": "app",
						},
					},
					MetricTags: map[string]*models.MetricTagValue{
						"app-guid": {
							Static: appGUID,
						},
					},
					Action: &models.Action{
						RunAction: &models.RunAction{
							Path: "/bin/bash",
							Args: []string{"-c", "echo sleeping; sleep 600"},
							User: "root",
						},
					},
					Ports: []uint32{8080},
				})).To(Succeed())
			})

			By("Waiting for the LRP to be running", func() {
				Eventually(func() string {
					lrps, err := bbsClient.ActualLRPs(logger, "trace", models.ActualLRPFilter{
						Domain:      "cf",
						ProcessGuid: processGUID,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(lrps).To(HaveLen(1))

					return lrps[0].State
				}, "5m", "10s").To(Equal(models.ActualLRPStateRunning))
			})

			By("Expecting the LRP to keep running", func() {
				time.Sleep(30 * time.Second)
				lrps, err := bbsClient.ActualLRPs(logger, "trace", models.ActualLRPFilter{
					Domain:      "cf",
					ProcessGuid: processGUID,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(lrps).To(HaveLen(1))
				Expect(lrps[0].State).To(Equal(models.ActualLRPStateRunning))
			})
		})

		It("handles failing LRPs", func() {
			Expect(bbsClient.DesireLRP(logger, "trace", &models.DesiredLRP{
				ProcessGuid: processGUID,
				Domain:      "cf",
				RootFs:      "preloaded:cflinuxfs4",
				Instances:   1,
				DiskMb:      256,
				MemoryMb:    64,
				Network: &models.Network{
					Properties: map[string]string{
						"app_id":             appGUID,
						"container_workload": "app",
					},
				},
				MetricTags: map[string]*models.MetricTagValue{
					"app-guid": {
						Static: appGUID,
					},
				},
				Action: &models.Action{
					RunAction: &models.RunAction{
						Path: "/bin/bash",
						Args: []string{"-c", "echo failing; exit 1"},
						User: "root",
					},
				},
				Ports: []uint32{8080},
			})).To(Succeed())

			Eventually(func() string {
				lrps, err := bbsClient.ActualLRPs(logger, "trace", models.ActualLRPFilter{
					Domain:      "cf",
					ProcessGuid: processGUID,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(lrps).To(HaveLen(1))

				return lrps[0].State
			}, "5m", "10s").To(Equal(models.ActualLRPStateCrashed))
		})
	})

	Describe("DesireTask", func() {
		AfterEach(func() {
			Expect(bbsClient.DeleteTask(logger, "trace", processGUID)).To(Succeed())
		})

		It("Creates a Pod from a Task", func() {
			Expect(bbsClient.DesireTask(logger, "trace", processGUID, "cf", &models.TaskDefinition{
				RootFs:   "preloaded:cflinuxfs4",
				DiskMb:   256,
				MemoryMb: 64,
				Network: &models.Network{
					Properties: map[string]string{
						"app_id":             appGUID,
						"container_workload": "app",
					},
				},
				MetricTags: map[string]*models.MetricTagValue{
					"app-guid": {
						Static: appGUID,
					},
				},
				Action: &models.Action{
					RunAction: &models.RunAction{
						Path: "/bin/bash",
						Args: []string{"-c", "echo running; sleep 5"},
						User: "root",
					},
				},
			})).To(Succeed())

			Eventually(func() models.Task_State {
				task, err := bbsClient.TaskByGuid(logger, "trace", processGUID)
				Expect(err).ToNot(HaveOccurred())

				return task.GetState()
			}, "5m", "10s").To(Equal(models.Task_Completed))

			Expect(bbsClient.ResolvingTask(logger, "trace", processGUID)).To(Succeed())
		})
	})
})
