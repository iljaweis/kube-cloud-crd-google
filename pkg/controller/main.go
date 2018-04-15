package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	googlev1 "github.com/iljaweis/kube-cloud-crd-google/pkg/apis/google.cloudcrd.weisnix.org/v1"
	googleclientset "github.com/iljaweis/kube-cloud-crd-google/pkg/client/clientset/versioned"
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

func (c *Controller) ProjectCreatedOrUpdated(project *googlev1.Project) error {
	log.Debugf("processing created or updated project '%s-%s'", project.Namespace, project.Name)
	return nil
}

func (c *Controller) ProjectDeleted(project *googlev1.Project) error {
	log.Debugf("processing deleted project '%s-%s'", project.Namespace, project.Name)
	return nil
}

func (c *Controller) InstanceCreatedOrUpdated(instance *googlev1.Instance) error {
	log.Debugf("processing created or updated instance '%s-%s'", instance.Namespace, instance.Name)

	var projectName string
	if projectName = instance.Spec.Project; projectName == "" {
		projectName = "default"
	}

	project, err := c.ProjectLister.Projects(instance.Namespace).Get(projectName)
	if err != nil {
		return fmt.Errorf("error getting project '%s-%s': %s", instance.Namespace, projectName, err.Error())
	}

	comp, err := c.ComputeService(projectName, instance.Namespace)
	if err != nil {
		return err
	}

	notfound := false
	inst, err := comp.Instances.Get(project.Spec.Name, project.Spec.Zone, instance.Name).Do()
	if err != nil {
		if gerr, ok := err.(*googleapi.Error); ok {
			// TODO: better way to handle NotFound?
			if gerr.Code == 404 {
				notfound = true
			} else {
				return fmt.Errorf("error getting instance '%s': %s", instance.Name, gerr.Error())
			}
		} else {
			return fmt.Errorf("error getting instance '%s': %s", instance.Name, err.Error())
		}
	}

	if notfound {
		log.Debugf("instance '%s' not found", instance.Name)

		i := compute.Instance{
			Name:           instance.Name,
			MinCpuPlatform: "Automatic",
			MachineType:    fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", project.Spec.Name, project.Spec.Zone, instance.Spec.Type),
			//Metadata: &compute.Metadata{
			//	Items: []*compute.MetadataItems{{
			//		Key: "ssh-keys",
			//		Value: &ssh_key,
			//	},
			//	}},
			//Tags: &compute.Tags{
			//	Items: []string{"sometag"},
			//},
			Disks: []*compute.AttachedDisk{{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					// TODO: Nicer way to choose image.
					SourceImage: instance.Spec.Image,
					DiskSizeGb:  instance.Spec.DiskSize,
				},
			}},
			NetworkInterfaces: []*compute.NetworkInterface{
				&compute.NetworkInterface{
					// TODO: configurable
					AccessConfigs: []*compute.AccessConfig{
						&compute.AccessConfig{
							Type: "ONE_TO_ONE_NAT",
							Name: "External NAT",
						},
					},
					Subnetwork: fmt.Sprintf("projects/%s/regions/%s/subnetworks/default", project.Spec.Name, project.Spec.Region),
				},
			},
			ServiceAccounts: []*compute.ServiceAccount{
				{
					Email: project.Spec.ServiceAccount,
					Scopes: []string{
						compute.DevstorageFullControlScope,
						compute.ComputeScope,
					},
				},
			},
		}

		_, err := comp.Instances.Insert(project.Spec.Name, project.Spec.Zone, &i).Do()
		if err != nil {
			return c.MakeEventAndFail(&instance.ObjectMeta, "Instance", fmt.Sprintf("could not create instance '%s': %s", instance.Name, err.Error()))
		} else {
			c.MakeEvent(&instance.ObjectMeta, "Instance", fmt.Sprintf("requested provisioning of instance '%s'", instance.Name), false)
			return nil
		}

	} else {
		log.Debugf("instance '%s' found", inst.Name)
		// TODO: Check status of instance. Any values that could be changed for a running instance?
	}

	return nil
}

func (c *Controller) InstanceDeleted(instance *googlev1.Instance) error {
	log.Debugf("processing deleted instance '%s-%s'", instance.Namespace, instance.Name)
	// TODO: Use a finalizer.

	var projectName string
	if projectName = instance.Spec.Project; projectName == "" {
		projectName = "default"
	}

	project, err := c.ProjectLister.Projects(instance.Namespace).Get(projectName)
	if err != nil {
		return fmt.Errorf("error getting project '%s-%s': %s", instance.Namespace, projectName, err.Error())
	}

	comp, err := c.ComputeService(projectName, instance.Namespace)
	if err != nil {
		return err
	}

	_, err = comp.Instances.Delete(project.Spec.Name, project.Spec.Zone, instance.Name).Do()
	if err != nil {
		return c.MakeEventAndFail(&instance.ObjectMeta, "Instance", fmt.Sprintf("could not delete instance '%s': %s", instance.Name, err.Error()))
	} else {
		c.MakeEvent(&instance.ObjectMeta, "Instance", fmt.Sprintf("requested deletion of instance '%s'", instance.Name), false)
		return nil
	}

	return nil
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
