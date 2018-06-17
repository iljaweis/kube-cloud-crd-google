package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"

	googlev1 "github.com/iljaweis/kube-cloud-crd-google/pkg/apis/google.cloudcrd.weisnix.org/v1"
)

func (c *Controller) InstanceCreatedOrUpdated(instance *googlev1.Instance) error {
	log.Debugf("processing created or updated instance '%s/%s'", instance.Namespace, instance.Name)

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
	log.Debugf("processing deleted instance '%s/%s'", instance.Namespace, instance.Name)
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

