package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const CLUSTER_NAME string = "local_demo_cluster"
const APP_LABEL string = "app.kubernetes.io/instance"

var gitCache = map[string][]string{}

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

	// List all deployments having argocd label app.kubernetes.io/instance
	// Use these deployments to get images and gather deploytime and commit time

	imagesFromDeployments := []string{}
	commitHashSet := map[string]bool{}

	deploymentList, err := collector.kubeClient.ListDeploymentsByLabels()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, depl := range deploymentList.Items {
		for _, cont := range depl.Spec.Template.Spec.Containers {
			imagesFromDeployments = append(imagesFromDeployments, cont.Image)
			if strings.HasPrefix(cont.Image, "quay.io/redhat-appstudio/") || strings.HasPrefix(cont.Image, "quay.io/stolostron/") || strings.HasPrefix(cont.Image, "quay.io/abarbaro/") {
				fmt.Println("collecting data for image ", cont.Image)
				namespace := depl.Namespace
				component := depl.Labels[APP_LABEL]

				fields := reSubMatchMap(imageRegex, cont.Image)

				commit, err := collector.githubClient.GetCommitFromOrgAndRepo(fields["org"], fields["repo"], fields["hash"])
				if err != nil {
					commit, err = collector.githubClient.SearchCommit(fields["hash"])
					if err != nil {
						fmt.Println("Can't find commit either by get or search: ", fields["repo"], " ", fields["hash"])
					}
				}

				_, ok := commitHashSet[cont.Image]
				if !ok {
					if err == nil {
						m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commit.Author.Date.UnixMilli()), component, fields["hash"], cont.Image, namespace)
						m1 = prometheus.NewMetricWithTimestamp(time.Now(), m1)
						ch <- m1
					}

					// If the deployment is active we also collect the deploy time metric using the deployment creation timestamp
					isActive, lastUpdate := collector.kubeClient.IsDeploymentActiveSince(&depl)
					if isActive {
						m1 := prometheus.MustNewConstMetric(collector.deployTimeMetric, prometheus.GaugeValue, float64(lastUpdate.UnixMilli()), component, fields["hash"], cont.Image, namespace)
						m1 = prometheus.NewMetricWithTimestamp(time.Now(), m1)
						ch <- m1
					} else {
						fmt.Printf("%s deploy time not collected because deployment is not in active state.\n", cont.Image)
					}

				}

				commitHashSet[cont.Image] = true
			} else {
				fmt.Printf("%s image is filtered out because not a redhat-appstudio one.\n", cont.Image)
			}
		}

	}

}

// Helper function and regex to extract values from an image URL
var imageRegex = regexp.MustCompile(`quay.io\/(?P<org>[-a-zA-Z0-9]*)\/(?P<repo>[-a-zA-Z0-9]*)(@sha256)?:(?P<hash>[-a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]*)`)

func reSubMatchMap(r *regexp.Regexp, str string) map[string]string {
	match := r.FindStringSubmatch(str)
	subMatchMap := make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i != 0 {
			subMatchMap[name] = match[i]
		}
	}
	return subMatchMap
}
