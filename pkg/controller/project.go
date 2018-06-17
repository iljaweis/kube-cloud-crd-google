package main

import (
	log "github.com/sirupsen/logrus"

	googlev1 "github.com/iljaweis/kube-cloud-crd-google/pkg/apis/google.cloudcrd.weisnix.org/v1"
)

func (c *Controller) ProjectCreatedOrUpdated(project *googlev1.Project) error {
	log.Debugf("processing created or updated project '%s/%s'", project.Namespace, project.Name)
	return nil
}

func (c *Controller) ProjectDeleted(project *googlev1.Project) error {
	log.Debugf("processing deleted project '%s/%s'", project.Namespace, project.Name)
	return nil
}

