/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gimlet-io/gimlet-cli/pkg/dashboard/api"
	log "github.com/sirupsen/logrus"
	networking_v1 "k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apps_v1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	api_v1 "k8s.io/api/core/v1"
	rntme "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	name         string
	indexer      cache.Indexer
	queue        workqueue.RateLimitingInterface
	informer     cache.Controller
	eventHandler func(informerEvent Event, objectMeta meta_v1.ObjectMeta, obj interface{}) error
	kubeEnv      *KubeEnv
}

// Event indicate the informerEvent
type Event struct {
	key       string
	eventType string
}

var serverStartTime time.Time

// NewController creates a new Controller.
func NewController(
	name string,
	listWatcher cache.ListerWatcher,
	objType rntme.Object,
	eventHandler func(informerEvent Event, objectMeta meta_v1.ObjectMeta, obj interface{}) error,
) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var informerEvent Event
	var err error

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the pod key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Pod than the version which was responsible for triggering the update.
	indexer, informer := cache.NewIndexerInformer(listWatcher, objType, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			informerEvent.key, err = cache.MetaNamespaceKeyFunc(obj)
			informerEvent.eventType = "create"
			if err == nil {
				queue.Add(informerEvent)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			informerEvent.key, err = cache.MetaNamespaceKeyFunc(old)
			informerEvent.eventType = "update"
			if err == nil {
				queue.Add(informerEvent)
			}
		},
		DeleteFunc: func(obj interface{}) {
			informerEvent.key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			informerEvent.eventType = "delete"
			if err == nil {
				queue.Add(informerEvent)
			}
		},
	}, cache.Indexers{})

	return &Controller{
		name:         name,
		informer:     informer,
		indexer:      indexer,
		queue:        queue,
		eventHandler: eventHandler,
	}
}

// NewController creates a new Controller.
func NewDynamicController(
	name string,
	dynamicClient dynamic.Interface,
	resource schema.GroupVersionResource,
	eventHandler func(informerEvent Event, objectMeta meta_v1.ObjectMeta, obj interface{}) error,
) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var informerEvent Event
	var err error

	dynInformer := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	informer := dynInformer.ForResource(resource).Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			informerEvent.key, err = cache.MetaNamespaceKeyFunc(obj)
			informerEvent.eventType = "create"
			if err == nil {
				queue.Add(informerEvent)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			informerEvent.key, err = cache.MetaNamespaceKeyFunc(old)
			informerEvent.eventType = "update"
			if err == nil {
				queue.Add(informerEvent)
			}
		},
		DeleteFunc: func(obj interface{}) {
			informerEvent.key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			informerEvent.eventType = "delete"
			if err == nil {
				queue.Add(informerEvent)
			}
		},
	})

	indexer := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})

	return &Controller{
		name:         name,
		informer:     informer,
		indexer:      indexer,
		queue:        queue,
		eventHandler: eventHandler,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	informerEvent, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(informerEvent)

	obj, _, err := c.indexer.GetByKey(informerEvent.(Event).key)
	if err != nil {
		log.Errorf("Fetching object with key %s from store failed with %v", informerEvent.(Event).key, err)
		return true
	}

	objectMeta := getObjectMetaData(obj)

	// don't process events from before agent start
	// startup sends the complete cluster state upstream
	if informerEvent.(Event).eventType == "create" &&
		objectMeta.CreationTimestamp.Sub(serverStartTime).Seconds() < 0 {
		return true
	}

	// Invoke the method containing the business logic
	err = c.eventHandler(informerEvent.(Event), objectMeta, obj)
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, informerEvent)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		log.Infof("Error syncing pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	log.Infof("Dropping pod %q out of the queue: %v", key, err)
}

// Run begins watching and syncing.
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	log.Infof("Starting %s controller", c.name)
	serverStartTime = time.Now().Local()

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	log.Infof("Stopping %s controller", c.name)
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func getObjectMetaData(obj interface{}) meta_v1.ObjectMeta {

	var objectMeta meta_v1.ObjectMeta

	switch object := obj.(type) {
	case *apps_v1.Deployment:
		objectMeta = object.ObjectMeta
	case *api_v1.ReplicationController:
		objectMeta = object.ObjectMeta
	case *apps_v1.ReplicaSet:
		objectMeta = object.ObjectMeta
	case *apps_v1.DaemonSet:
		objectMeta = object.ObjectMeta
	case *api_v1.Service:
		objectMeta = object.ObjectMeta
	case *api_v1.Pod:
		objectMeta = object.ObjectMeta
	case *batch_v1.Job:
		objectMeta = object.ObjectMeta
	case *api_v1.PersistentVolume:
		objectMeta = object.ObjectMeta
	case *api_v1.Namespace:
		objectMeta = object.ObjectMeta
	case *api_v1.Secret:
		objectMeta = object.ObjectMeta
	case *networking_v1.Ingress:
		objectMeta = object.ObjectMeta
	}
	return objectMeta
}

func sendUpdate(host string, agentKey string, env string, update interface{}) {
	stacksString, err := json.Marshal(update)
	if err != nil {
		log.Errorf("could not serialize k8s state: %v", err)
		return
	}

	req, err := http.NewRequest("POST", host+"/agent/state/"+env+"/update", bytes.NewBuffer(stacksString))
	if err != nil {
		log.Errorf("could not create http request: %v", err)
		return
	}
	req.Header.Set("Authorization", "BEARER "+agentKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("could not send state update: %d - %v", resp.StatusCode, string(body))
		return
	}
}

func sendEvents(host string, agentKey string, events []api.Event) {
	typeWarningEventsString, err := json.Marshal(events)
	if err != nil {
		log.Errorf("could not serialize k8s events: %v", err)
		return
	}

	reqUrl := fmt.Sprintf("%s/agent/events", host)
	req, err := http.NewRequest("POST", reqUrl, bytes.NewBuffer(typeWarningEventsString))
	if err != nil {
		log.Errorf("could not create http request: %v", err)
		return
	}
	req.Header.Set("Authorization", "BEARER "+agentKey)
	req.Header.Set("Content-Type", "application/json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("could not send k8s events: %d - %v", resp.StatusCode, string(body))
		return
	}
}
