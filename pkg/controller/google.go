package main

import (
	"fmt"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2"
	"net/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) NewGoogleClient(projectName, namespace, scope string) (*http.Client, error) {
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

	conf, err := google.JWTConfigFromJSON(cred, scope)
	if err != nil {
		return nil, fmt.Errorf("error creating authentication for project '%s-%s': %s", namespace, projectName, err.Error())
	}

	client := conf.Client(oauth2.NoContext)

	return client, nil
}