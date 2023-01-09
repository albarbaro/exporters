package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

const CLUSTER_NAME string = "local_demo_cluster"
const APP_LABEL string = "app.kubernetes.io/instance"

// Define a struct for you collector that contains pointers to prometheus descriptors for each metric you wish to expose.
// You can also include fields of other types if they provide utility
type CommitTimeCollector struct {
	commitTimeMetric *prometheus.Desc
	deployTimeMetric *prometheus.Desc
	githubClient     *GithubClient
	kubeClient       *KubeClients
	commitHashSet    map[string]bool
	gitCache         map[string]*time.Time
	searchLabel      string
	imageFilter      []string
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

	// Get configmap and set config parameters

	// default values: if not specified in the config, use these values
	searchLabel := "app.kubernetes.io/instance"
	imageFilters := []string{"quay.io/redhat-appstudio/", "quay.io/redhat-appstudio-qe/", "quay.io/stolostron/", "quay.io/abarbaro/"}
	configMap, err := kubeClient.GetConfigMap("exporters-config", "dora-metrics")
	if err == nil {
		filtersJson := configMap.Data["imageFilters"]
		var imageFilters_ []string
		err = json.Unmarshal([]byte(filtersJson), &imageFilters_)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal config from configmap")
		}

		if label, ok := configMap.Data["searchLabel"]; ok {
			searchLabel = label
		}
	} else {
		fmt.Println("no configmap found")
	}

	fmt.Println("Using label ", searchLabel)
	fmt.Println("Using image filters: ", imageFilters)

	return &CommitTimeCollector{
		commitTimeMetric: prometheus.NewDesc("committime_metric",
			"Shows timestamp for a specific commit",
			[]string{"app", "commit_hash", "image_sha", "namespace"}, nil,
		),
		deployTimeMetric: prometheus.NewDesc("deploytime_metric",
			"Shows deployment timestamp for a specific commit",
			[]string{"app", "commit_hash", "image_sha", "namespace"}, nil,
		),
		githubClient:  gh,
		kubeClient:    kubeClient,
		commitHashSet: map[string]bool{},
		gitCache:      map[string]*time.Time{},
		searchLabel:   searchLabel,
		imageFilter:   imageFilters,
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

	// this map is used at each scraping to avoid parsing an image twice: duplicate metrics are not allowed by the collector and will throw an error
	collector.commitHashSet = map[string]bool{}

	deploymentList, err := collector.kubeClient.ListDeploymentsByLabels(collector.searchLabel)
	if err != nil {
		fmt.Println(err)
		return
	}

	// loop through all deployments found
	for _, depl := range deploymentList.Items {
		// and get all container's images
		for _, cont := range depl.Spec.Template.Spec.Containers {
			// filter the images to only use the appstudio ones
			if isOk := filterImage(collector.imageFilter, cont.Image); isOk {
				collector.CollectCommitTime(ch, &depl, &cont)
				collector.CollectDeployTime(ch, &depl, &cont)
				collector.commitHashSet[cont.Image] = true
			}
		}
	}

}

func (collector *CommitTimeCollector) CollectCommitTime(ch chan<- prometheus.Metric, depl *appsv1.Deployment, cont *v1.Container) {
	// check we have not parsed this image already
	_, ok := collector.commitHashSet[cont.Image]
	if !ok {
		// get data needed for prometheus labels
		namespace := depl.Namespace
		component := depl.Labels[collector.searchLabel]
		// parse the image url to extract organization, repository and commit hash
		fields := reSubMatchMap(imageRegex, cont.Image)

		// check if we have already searched for this commit, before requesting github apis
		// if yes, use that value and return
		commitTimeValue, commitCached := collector.gitCache[cont.Image]
		if commitCached {
			m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commitTimeValue.Unix()), component, fields["hash"], cont.Image, namespace)
			m1 = prometheus.NewMetricWithTimestamp(*commitTimeValue, m1)
			ch <- m1
			//fmt.Println("collected (from cache) committime for ", cont.Image)
			return
		}
		//fmt.Println("Commit time is not cached yet: ", fields["repo"], " ", fields["hash"])

		// if the data is not cached, look in github: first try is using org+repo+hash to directly get the data from the repo (we want to avoid searching for a generic hash)
		commit, err := collector.githubClient.GetCommitFromOrgAndRepo(fields["org"], fields["repo"], fields["hash"])
		if err == nil {
			m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commit.Author.Date.Unix()), component, fields["hash"], cont.Image, namespace)
			m1 = prometheus.NewMetricWithTimestamp(*commit.Author.Date, m1)
			ch <- m1
			//fmt.Println("collected committime for ", cont.Image)
			collector.gitCache[cont.Image] = commit.Author.Date
			return
		}
		//fmt.Println("Can't find commit time using ", fields["repo"], " and ", fields["hash"])

		commit, err = collector.githubClient.SearchCommit(fields["hash"])
		if err == nil {
			m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commit.Author.Date.Unix()), component, fields["hash"], cont.Image, namespace)
			m1 = prometheus.NewMetricWithTimestamp(*commit.Author.Date, m1)
			ch <- m1
			//fmt.Println("collected committime for ", cont.Image, ": ", err)
			collector.gitCache[cont.Image] = commit.Author.Date
			return
		}
		fmt.Println("Can't find commit either by get or search: ", fields["repo"], " ", fields["hash"])

	}

}

func (collector *CommitTimeCollector) CollectDeployTime(ch chan<- prometheus.Metric, depl *appsv1.Deployment, cont *v1.Container) {
	// check we have not parsed this image already
	_, ok := collector.commitHashSet[cont.Image]
	if !ok {
		// get data needed for prometheus labels
		namespace := depl.Namespace
		component := depl.Labels[collector.searchLabel]
		// parse the image url to extract organization, repository and commit hash
		fields := reSubMatchMap(imageRegex, cont.Image)
		// If the deployment is active we also collect the deploy time metric using the deployment creation timestamp
		isActive, _ := collector.kubeClient.IsDeploymentActiveSince(depl)
		if isActive {
			creationTime, err := collector.kubeClient.GetDeploymentReplicaSetCreationTime(namespace, depl.Name, cont.Image)
			if err != nil {
				fmt.Println(err)
			} else {
				m1 := prometheus.MustNewConstMetric(collector.deployTimeMetric, prometheus.GaugeValue, float64(creationTime.Unix()), component, fields["hash"], cont.Image, namespace)
				m1 = prometheus.NewMetricWithTimestamp(creationTime.Time, m1)
				ch <- m1
				//fmt.Println("collected deploytime for ", cont.Image)
			}

		} else {
			fmt.Printf("%s deploy time not collected because deployment is not in active state.\n", cont.Image)
		}
	}
}
