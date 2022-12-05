package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const CLUSTER_NAME string = "local_demo_cluster"

// Define a struct for you collector that contains pointers to prometheus descriptors for each metric you wish to expose.
// You can also include fields of other types if they provide utility
type CommitTimeCollector struct {
	commitTimeMetric *prometheus.Desc
	deployTimeMetric *prometheus.Desc
	githubClient     *GithubClient
	kubeClient       *KubeClients
}

// You must create a constructor for you collector that initializes every descriptor and returns a pointer to the collector
func NewCommitTimeCollector() (*CommitTimeCollector, error) {
	// Initialize the github client
	gh, err := NewGithubClient()
	if err != nil {
		return nil, err
	}

	// Initialize the kubernetes clients (clientset and rest)
	kubeClient, err := NewKubeClient()
	if err != nil {
		return nil, err
	}

	return &CommitTimeCollector{
		commitTimeMetric: prometheus.NewDesc("committime_metric",
			"Shows timestamp for a specific commit",
			[]string{"app", "commit_hash", "image_sha", "namespace"}, nil,
		),
		deployTimeMetric: prometheus.NewDesc("deploytime_metric",
			"Shows deployment timestamp for a specific commit",
			[]string{"app", "commit_hash", "image_sha", "namespace"}, nil,
		),
		githubClient: gh,
		kubeClient:   kubeClient,
	}, nil
}

//Each and every collector must implement the Describe function. It essentially writes all descriptors to the prometheus desc channel.
func (collector *CommitTimeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.commitTimeMetric
	ch <- collector.deployTimeMetric
}

//Collect implements required collect function for all promehteus collectors
func (collector *CommitTimeCollector) Collect(ch chan<- prometheus.Metric) {

	// List all ArgoCD apps in the cluster
	// TDB: maybe fiilter by namespace?
	// Note: also availbale ListArgoCDAppsByLabels if labels are available
	argocdApps, err := collector.kubeClient.ListArgoCDApps()
	if err != nil {
		fmt.Println(err)
	}

	for _, app := range argocdApps.Items {
		// Get a list of images from all the apps
		images, err := collector.kubeClient.GetImagesFromArgoCDApp(&app)
		if err != nil {
			fmt.Println(err)
		}

		for _, image := range images {
			// Filter images here if you need to
			if strings.HasPrefix(image, "quay.io/redhat-appstudio/") {

				imageName := strings.Split(image, ":")[0]
				commitHash := strings.Split(image, ":")[1]

				commit, err := collector.githubClient.SearchCommit(commitHash)
				if err != nil {
					fmt.Println(err)
					break
				}

				namespace := app.Spec.Destination.Namespace
				component := strings.Split(app.Spec.Source.Path, "/")[1]

				m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commit.Author.Date.UnixNano()), component, commitHash, imageName, namespace)
				m1 = prometheus.NewMetricWithTimestamp(time.Now(), m1)
				ch <- m1
			}
		}
	}

	deploymentList, err := collector.kubeClient.ListDeploymentsByLabels()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, depl := range deploymentList.Items {
		if collector.kubeClient.IsDeploymentActive(&depl) {
			for _, cont := range depl.Spec.Template.Spec.Containers {
				if strings.HasPrefix(cont.Image, "quay.io/redhat-appstudio/") {

					imageName := strings.Split(cont.Image, ":")[0]
					commitHash := strings.Split(cont.Image, ":")[1]
					namespace := depl.Namespace
					component := depl.Labels["app.kubernetes.io/instance"]

					m1 := prometheus.MustNewConstMetric(collector.deployTimeMetric, prometheus.GaugeValue, float64(depl.CreationTimestamp.UnixNano()), component, commitHash, imageName, namespace)
					m1 = prometheus.NewMetricWithTimestamp(time.Now(), m1)
					ch <- m1
				}
			}
		}
	}
}
