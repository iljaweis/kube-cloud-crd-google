package main

import (

	log "github.com/sirupsen/logrus"

googlev1 "github.com/iljaweis/kube-cloud-crd-google/pkg/apis/google.cloudcrd.weisnix.org/v1"

	"fmt"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/sqladmin/v1beta4"
)

func (c *Controller) DatabaseCreatedOrUpdated(database *googlev1.Database) error {
	log.Debugf("processing created or updated database '%s/%s'", database.Namespace, database.Name)

	var projectName string
	if projectName = database.Spec.Project; projectName == "" {
		projectName = "default"
	}

	project, err := c.ProjectLister.Projects(database.Namespace).Get(projectName)
	if err != nil {
		return fmt.Errorf("error getting project '%s-%s': %s", database.Namespace, projectName, err.Error())
	}

	sqla, err := c.SqladminService(projectName, database.Namespace)
	if err != nil {
		return err
	}

	comp, err := c.ComputeService(projectName, database.Namespace)
	if err != nil {
		return err
	}

	computeInstances, err := comp.Instances.List(project.Spec.Name, project.Spec.Zone).Do()
	if err != nil {
		return err
	}

	authNets := make([]*sqladmin.AclEntry, len(computeInstances.Items))
	for i, in := range computeInstances.Items {
		for _, iface := range in.NetworkInterfaces {
			for _, ac := range iface.AccessConfigs {
				authNets[i] = &sqladmin.AclEntry{Value: ac.NatIP}
			}
		}
	}

	notfound := false
	inst, err := sqla.Instances.Get(project.Spec.Name, database.Name).Do()
	if err != nil {
		if gerr, ok := err.(*googleapi.Error); ok {
			// TODO: better way to handle NotFound?
			if gerr.Code == 404 {
				notfound = true
			} else if gerr.Code == 403 {
				notfound = true // we do not get 404 when the database does not exist for some reason
			} else {
				return fmt.Errorf("error getting database '%s': %s", database.Name, gerr.Error())
			}
		} else {
			return fmt.Errorf("error getting database '%s': %s", database.Name, err.Error())
		}
	}

	if notfound {
		log.Debugf("database '%s' not found", database.Name)

		db := sqladmin.DatabaseInstance{
			Name: database.Name,
			BackendType: "SECOND_GEN",
			DatabaseVersion: "MYSQL_5_7",
			Settings: &sqladmin.Settings{
				Tier: "db-n1-standard-1",
				IpConfiguration: &sqladmin.IpConfiguration{
					AuthorizedNetworks: authNets,
				},
			},
		}

		_, err := sqla.Instances.Insert(project.Spec.Name, &db).Do()
		if err != nil {
			return c.MakeEventAndFail(&database.ObjectMeta, "database", fmt.Sprintf("could not create database '%s': %s", database.Name, err.Error()))
		} else {
			c.MakeEvent(&database.ObjectMeta, "database", fmt.Sprintf("requested provisioning of database '%s'", database.Name), false)
			return nil
		}

		// TODO: create password and store in Secret
		// TODO: create ConfigMap and/or Service with database address

	} else {
		log.Debugf("database '%s' found", inst.Name)
		// TODO: Check status of database. Any values that could be changed for a running database?
	}

	return nil
}

func (c *Controller) DatabaseDeleted(database *googlev1.Database) error {
	log.Debugf("processing deleted database '%s/%s'", database.Namespace, database.Name)
	// TODO: Use a finalizer.

	var projectName string
	if projectName = database.Spec.Project; projectName == "" {
		projectName = "default"
	}

	project, err := c.ProjectLister.Projects(database.Namespace).Get(projectName)
	if err != nil {
		return fmt.Errorf("error getting project '%s-%s': %s", database.Namespace, projectName, err.Error())
	}

	sqla, err := c.SqladminService(projectName, database.Namespace)
	if err != nil {
		return err
	}

	_, err = sqla.Instances.Delete(project.Spec.Name, database.Name).Do()
	if err != nil {
		return c.MakeEventAndFail(&database.ObjectMeta, "database", fmt.Sprintf("could not delete database '%s': %s", database.Name, err.Error()))
	} else {
		c.MakeEvent(&database.ObjectMeta, "database", fmt.Sprintf("requested deletion of database '%s'", database.Name), false)
		return nil
	}

	return nil
}

