package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	googleclientset "github.com/iljaweis/kube-cloud-crd-google/pkg/client/clientset/versioned"
	"google.golang.org/api/sqladmin/v1beta4"
)

func main() {

	log.SetLevel(log.DebugLevel)

	var kubeconfig string

	if e := os.Getenv("KUBECONFIG"); e != "" {
		kubeconfig = e
	}

	flag.StringVar(&kubeconfig, "kubeconfig", os.Getenv("HOME")+"/.kube/config", "location of your kubeconfig")
	flag.Parse()

	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err.Error())
	}

	google, err := googleclientset.NewForConfig(clientConfig)
	if err != nil {
		panic(err.Error())
	}

	c := &Controller{Kubernetes: clientset, GoogleClient: google}

	c.Initialize()
	c.Start()
}

func (c *Controller) MakeEvent(meta *metav1.ObjectMeta, kind string, message string, warn bool) error {
	var t string
	if warn {
		t = "Warning"
	} else {
		t = "Normal"
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: meta.Name,
		},
		InvolvedObject: corev1.ObjectReference{
			Name:            meta.Name,
			Namespace:       meta.Namespace,
			APIVersion:      "v1",
			UID:             meta.GetUID(),
			Kind:            kind,
			ResourceVersion: meta.ResourceVersion,
		},
		Message:        message,
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		Type:           t,
	}

	_, err := c.Kubernetes.CoreV1().Events(meta.Namespace).Create(event)
	return err
}

func (c *Controller) MakeEventAndFail(meta *metav1.ObjectMeta, kind string, message string) error {
	log.Error(message)
	_ = c.MakeEvent(meta, kind, message, true)
	return fmt.Errorf(message)
}

func (c *Controller) ComputeService(projectName string, namespace string) (*compute.Service, error) {
	project, err := c.ProjectLister.Projects(namespace).Get(projectName)
	if err != nil {
		return nil, fmt.Errorf("error getting project '%s-%s': %s", namespace, projectName, err.Error())
	}

	secret, err := c.Kubernetes.CoreV1().Secrets(namespace).Get(project.Spec.ServiceAccountSecret, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting secret '%s-%s' for serviceaccount of project '%s-%s': %s", namespace, project.Spec.ServiceAccountSecret, namespace, projectName, err.Error())
	}

	cred := secret.Data["json"]
	if len(cred) == 0 {
		return nil, fmt.Errorf("secret '%s-%s' for serviceaccount of project '%s-%s' does not contain a field 'json'", namespace, project.Spec.ServiceAccountSecret, namespace, projectName)
	}

	conf, err := google.JWTConfigFromJSON(cred, "https://www.googleapis.com/auth/compute")
	if err != nil {
		return nil, fmt.Errorf("error creating authentication for project '%s-%s': %s", namespace, projectName, err.Error())
	}

	client := conf.Client(oauth2.NoContext)

	comp, err := compute.New(client)
	if err != nil {
		return nil, fmt.Errorf("error creating compute client for project '%s-%s': %s", namespace, projectName, err.Error())
	}

	return comp, nil
}

func (c *Controller) SqladminService(projectName string, namespace string) (*sqladmin.Service, error) {
	client, err := c.NewGoogleClient(projectName, namespace, "https://www.googleapis.com/auth/sqlservice.admin")
	if err != nil {
		return nil, err
	}

	sqla, err := sqladmin.New(client)
	if err != nil {
		return nil, fmt.Errorf("error creating sqladmin client for project '%s-%s': %s", namespace, projectName, err.Error())
	}

	return sqla, nil
}
