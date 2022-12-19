package main

import (
	"context"

	argocd "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(argocd.AddToScheme(scheme))
}

type KubeClients struct {
	kubeClient *kubernetes.Clientset
	crClient   crclient.Client
}

func NewKubeClient() (*KubeClients, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	crClient, err := crclient.New(cfg, crclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return &KubeClients{
		kubeClient: kclient,
		crClient:   crClient,
	}, nil
}

func (k *KubeClients) Clientset() *kubernetes.Clientset {
	return k.kubeClient
}

func (k *KubeClients) REST() crclient.Client {
	return k.crClient
}

func (k *KubeClients) ListArgoCDApps() (*argocd.ApplicationList, error) {
	//labels := map[string]string{"app.kubernetes.io/instance": "all-components-staging"}

	list := &argocd.ApplicationList{}
	err := k.REST().List(context.TODO(), list, &crclient.ListOptions{})
	if err != nil {
		return nil, err
	}

	return list, nil
}

func (k *KubeClients) ListArgoCDAppsByLabels(labelMap map[string]string) (*argocd.ApplicationList, error) {

	list := &argocd.ApplicationList{}
	err := k.crClient.List(context.TODO(), list, &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap)})

	if err != nil {
		return nil, err
	}

	return list, nil
}

func (k *KubeClients) GetImagesFromArgoCDApp(app *argocd.Application) ([]string, error) {
	images := []string{}
	images = append(images, app.Status.Summary.Images...)
	return images, nil
}

func (k *KubeClients) GetImagesFromArgoCDAppList(list *argocd.ApplicationList) ([]string, error) {
	images := []string{}

	for _, app := range list.Items {
		images = append(images, app.Status.Summary.Images...)
	}

	return images, nil
}

func (k *KubeClients) ListDeploymentsByLabels() (*appsv1.DeploymentList, error) {

	deploymentList, err := k.kubeClient.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/instance"})

	if err != nil {
		return nil, err
	}

	return deploymentList, nil
}

func (k *KubeClients) IsDeploymentActiveSince(deployment *appsv1.Deployment) (bool, metav1.Time) {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == "Available" && condition.Status == "True" {
			return true, condition.LastUpdateTime
		}
	}
	return false, metav1.Time{}
}
