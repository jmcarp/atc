package config_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Job config", func() {
	Describe("JobInputs", func() {
		var (
			jobConfig atc.JobConfig

			inputs []config.JobInput
		)

		BeforeEach(func() {
			jobConfig = atc.JobConfig{}
		})

		JustBeforeEach(func() {
			inputs = config.JobInputs(jobConfig)
		})

		Context("with a build plan", func() {
			Context("with an empty plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{}
				})

				It("returns an empty set of inputs", func() {
					Expect(inputs).To(BeEmpty())
				})
			})

			Context("with two serial gets", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get:     "some-get-plan",
							Passed:  []string{"a", "b"},
							Trigger: true,
						},
						{
							Get: "some-other-get-plan",
						},
					}
				})

				It("uses both for inputs", func() {
					Expect(inputs).To(Equal([]config.JobInput{
						{
							Name:     "some-get-plan",
							Resource: "some-get-plan",
							Passed:   []string{"a", "b"},
							Trigger:  true,
						},
						{
							Name:     "some-other-get-plan",
							Resource: "some-other-get-plan",
							Trigger:  false,
						},
					}))

				})
			})

			Context("when a plan has a version on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Version: &atc.VersionConfig{
								Every: true,
							},
						},
					}
				})

				It("returns an input config with the version", func() {
					Expect(inputs).To(Equal(
						[]config.JobInput{
							{
								Name:     "a",
								Resource: "a",
								Version: &atc.VersionConfig{
									Every: true,
								},
							},
						},
					))
				})
			})

			Context("when a plan has an ensure hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Ensure: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						config.JobInput{
							Name:     "a",
							Resource: "a",
						},
						config.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an success hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Success: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						config.JobInput{
							Name:     "a",
							Resource: "a",
						},
						config.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an failure hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Failure: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						config.JobInput{
							Name:     "a",
							Resource: "a",
						},
						config.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a resource is specified", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get:      "some-get-plan",
							Resource: "some-get-resource",
						},
					}
				})

				It("uses it as resource in the input config", func() {
					Expect(inputs).To(Equal([]config.JobInput{
						{
							Name:     "some-get-plan",
							Resource: "some-get-resource",
							Trigger:  false,
						},
					}))

				})
			})

			Context("when a simple aggregate plan is the first step", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{Get: "a"},
								{Put: "y"},
								{Get: "b", Resource: "some-resource", Passed: []string{"x"}},
								{Get: "c", Trigger: true},
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(Equal([]config.JobInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  false,
						},
						{
							Name:     "b",
							Resource: "some-resource",
							Passed:   []string{"x"},
							Trigger:  false,
						},
						{
							Name:     "c",
							Resource: "c",
							Trigger:  true,
						},
					}))

				})
			})

			Context("when an overly complicated aggregate plan is the first step", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{
									Aggregate: &atc.PlanSequence{
										{Get: "a"},
									},
								},
								{Get: "b", Resource: "some-resource", Passed: []string{"x"}},
								{Get: "c", Trigger: true},
							},
						},
					}
				})

				It("returns an input config for all of the get plans present", func() {
					Expect(inputs).To(Equal([]config.JobInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  false,
						},
						{
							Name:     "b",
							Resource: "some-resource",
							Passed:   []string{"x"},
							Trigger:  false,
						},
						{
							Name:     "c",
							Resource: "c",
							Trigger:  true,
						},
					}))

				})
			})

			Context("when there are not gets in the plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "some-put-plan",
						},
					}
				})

				It("returns an empty set of inputs", func() {
					Expect(inputs).To(BeEmpty())
				})
			})
		})
	})

	Describe("Outputs", func() {
		var (
			jobConfig atc.JobConfig

			outputs []config.JobOutput
		)

		BeforeEach(func() {
			jobConfig = atc.JobConfig{}
		})

		JustBeforeEach(func() {
			outputs = config.JobOutputs(jobConfig)
		})

		Context("with a build plan", func() {
			Context("with an empty plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{}
				})

				It("returns an empty set of outputs", func() {
					Expect(outputs).To(BeEmpty())
				})
			})

			Context("when an overly complicated plan is configured", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{
									Aggregate: &atc.PlanSequence{
										{Put: "a"},
									},
								},
								{Put: "b", Resource: "some-resource"},
								{
									Do: &atc.PlanSequence{
										{Put: "c"},
									},
								},
							},
						},
					}
				})

				It("returns an output for all of the put plans present", func() {
					Expect(outputs).To(Equal([]config.JobOutput{
						{
							Name:     "a",
							Resource: "a",
						},
						{
							Name:     "b",
							Resource: "some-resource",
						},
						{
							Name:     "c",
							Resource: "c",
						},
					}))

				})
			})

			Context("when a plan has an ensure on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Ensure: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						config.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						config.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an success hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Success: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						config.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						config.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an failure hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Failure: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						config.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						config.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when the plan contains no puts steps", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "some-put-plan",
						},
					}
				})

				It("returns an empty set of outputs", func() {
					Expect(outputs).To(BeEmpty())
				})
			})
		})
	})
})
