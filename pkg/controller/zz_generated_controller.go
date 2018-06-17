package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kubernetesinformers "k8s.io/client-go/informers"





	googleclientset "github.com/iljaweis/kube-cloud-crd-google/pkg/client/clientset/versioned"

	googlev1 "github.com/iljaweis/kube-cloud-crd-google/pkg/apis/google.cloudcrd.weisnix.org/v1"
googleinformers "github.com/iljaweis/kube-cloud-crd-google/pkg/client/informers/externalversions"
	googlelisterv1 "github.com/iljaweis/kube-cloud-crd-google/pkg/client/listers/google.cloudcrd.weisnix.org/v1"

)

type Controller struct {

	Kubernetes kubernetes.Interface
	KubernetesFactory kubernetesinformers.SharedInformerFactory



	GoogleClient googleclientset.Interface
	GoogleFactory googleinformers.SharedInformerFactory



	ProjectQueue workqueue.RateLimitingInterface
	ProjectLister googlelisterv1.ProjectLister
	ProjectSynced cache.InformerSynced

	InstanceQueue workqueue.RateLimitingInterface
	InstanceLister googlelisterv1.InstanceLister
	InstanceSynced cache.InformerSynced

	DatabaseQueue workqueue.RateLimitingInterface
	DatabaseLister googlelisterv1.DatabaseLister
	DatabaseSynced cache.InformerSynced





}

// Expects the clientsets to be set.
func (c *Controller) Initialize() {

	if c.Kubernetes == nil {
		panic("c.Kubernetes is nil")
	}
	c.KubernetesFactory = kubernetesinformers.NewSharedInformerFactory(c.Kubernetes, time.Second*30)



	if c.GoogleClient == nil {
		panic("c.GoogleClient is nil")
	}
	c.GoogleFactory = googleinformers.NewSharedInformerFactory(c.GoogleClient, time.Second*30)




	ProjectInformer := c.GoogleFactory.Google().V1().Projects()
	ProjectQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.ProjectQueue = ProjectQueue
	c.ProjectLister = ProjectInformer.Lister()
	c.ProjectSynced = ProjectInformer.Informer().HasSynced

	ProjectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				ProjectQueue.Add(key)
			}
		},


		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				ProjectQueue.Add(key)
			}
		},


		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*googlev1.Project)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*googlev1.Project)
				if !ok {
					log.Errorf("tombstone contained object that is not a Project %+v", obj)
					return
				}
			}

			err := c.ProjectDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},

	})



	InstanceInformer := c.GoogleFactory.Google().V1().Instances()
	InstanceQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.InstanceQueue = InstanceQueue
	c.InstanceLister = InstanceInformer.Lister()
	c.InstanceSynced = InstanceInformer.Informer().HasSynced

	InstanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				InstanceQueue.Add(key)
			}
		},


		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				InstanceQueue.Add(key)
			}
		},


		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*googlev1.Instance)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*googlev1.Instance)
				if !ok {
					log.Errorf("tombstone contained object that is not a Instance %+v", obj)
					return
				}
			}

			err := c.InstanceDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},

	})



	DatabaseInformer := c.GoogleFactory.Google().V1().Databases()
	DatabaseQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.DatabaseQueue = DatabaseQueue
	c.DatabaseLister = DatabaseInformer.Lister()
	c.DatabaseSynced = DatabaseInformer.Informer().HasSynced

	DatabaseInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				DatabaseQueue.Add(key)
			}
		},


		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				DatabaseQueue.Add(key)
			}
		},


		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*googlev1.Database)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*googlev1.Database)
				if !ok {
					log.Errorf("tombstone contained object that is not a Database %+v", obj)
					return
				}
			}

			err := c.DatabaseDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},

	})





	return
}

func (c *Controller) Start() {
	stopCh := make(chan struct{})
	defer close(stopCh)
go c.KubernetesFactory.Start(stopCh)
go c.GoogleFactory.Start(stopCh)

	go c.Run(stopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func (c *Controller) Run(stopCh <-chan struct{}) {

	log.Infof("starting controller")

	defer runtime.HandleCrash()

	defer c.ProjectQueue.ShutDown()
	defer c.InstanceQueue.ShutDown()
	defer c.DatabaseQueue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.ProjectSynced, c.InstanceSynced, c.DatabaseSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	log.Debugf("starting workers")


	go wait.Until(c.runProjectWorker, time.Second, stopCh)

	go wait.Until(c.runInstanceWorker, time.Second, stopCh)

	go wait.Until(c.runDatabaseWorker, time.Second, stopCh)


	log.Debugf("started workers")
	<-stopCh
	log.Debugf("shutting down workers")
}



func (c *Controller) runProjectWorker() {
	for c.processNextProject() {
	}
}

func (c *Controller) processNextProject() bool {
	obj, shutdown := c.ProjectQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.ProjectQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.ProjectQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processProject(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.ProjectQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processProject(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.ProjectLister.Projects(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.ProjectCreatedOrUpdated(o)

}

func (c *Controller) runInstanceWorker() {
	for c.processNextInstance() {
	}
}

func (c *Controller) processNextInstance() bool {
	obj, shutdown := c.InstanceQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.InstanceQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.InstanceQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processInstance(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.InstanceQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processInstance(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.InstanceLister.Instances(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.InstanceCreatedOrUpdated(o)

}

func (c *Controller) runDatabaseWorker() {
	for c.processNextDatabase() {
	}
}

func (c *Controller) processNextDatabase() bool {
	obj, shutdown := c.DatabaseQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.DatabaseQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.DatabaseQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processDatabase(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.DatabaseQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processDatabase(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.DatabaseLister.Databases(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.DatabaseCreatedOrUpdated(o)

}